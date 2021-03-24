package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"

	dgo "github.com/bwmarrin/discordgo"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

var (
	// Semaphore signalling when we are listening
	listening = make(chan struct{}, 1)
)

type speaker struct {
	uid     string
	ssrc    uint32
	file    media.Writer
	audioId int64
}

func listen(s *dgo.Session, vc *dgo.VoiceConnection, convId int64, duration time.Duration) {
	var (
		timeout      = time.NewTimer(duration)
		quit         = make(chan os.Signal, 1)
		disconnected = make(chan struct{})
		closedFiles  = make(chan struct{})
		newSpeaker   = make(chan *speaker)
	)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	stopHandling := s.AddHandler(func(s *dgo.Session, m *dgo.VoiceStateUpdate) {
		if m.UserID == vc.UserID {
			if m.ChannelID == "" {
				log.Printf("forcibly disconnected from '%s'\n",
					m.BeforeUpdate.ChannelID)
				disconnected <- struct{}{}
			} else {
				// TODO What should happen when we are moved to another channel?
				log.Printf("moved to %s", m.ChannelID)
			}
		}
	})
	vc.AddHandler(func(vc *dgo.VoiceConnection, vs *dgo.VoiceSpeakingUpdate) {
		// Experimentally, vs.Speaking is never false. Why?
		if !vs.Speaking {
			log.Printf("user '%s' not speaking\n", vs.UserID)
			return
		}

		if dbIsConsenting(vs.UserID) {
			log.Printf("user '%s' is speaking in channel '%s'\n", vs.UserID, vc.ChannelID)
			// Copy mediaRoot to be modified locally
			url := *mediaRoot
			switch url.Scheme {
			case "file":
				url.Path = path.Join(url.Path, fmt.Sprintf("%v-%s.ogg", convId, vs.UserID))
				writer, err := oggwriter.New(url.Path, 48000, 2)
				if err != nil {
					log.Panicln("failed to open ogg writer: ", err)
				}
				audioId := dbCreateAudio(vs.UserID, convId, url.String())
				newSpeaker <- &speaker{vs.UserID, uint32(vs.SSRC), writer, audioId}
			default:
				log.Printf("scheme %s not implemented", url.Scheme)
				newSpeaker <- &speaker{uid: vs.UserID, ssrc: uint32(vs.SSRC)}
			}
		} else {
			log.Printf("user '%s' is speaking in channel '%s' but did not give consent\n",
				vs.UserID, vc.ChannelID)
			newSpeaker <- &speaker{uid: vs.UserID, ssrc: uint32(vs.SSRC)}
		}
	})

	defer func() {
		stopHandling()
		if err := vc.Disconnect(); err != nil {
			log.Println(err)
		}
		signal.Stop(quit)
		timeout.Stop()
		close(vc.OpusRecv)
		close(newSpeaker)
		dbEndConversation(convId)
		<-closedFiles
		<-listening
	}()

	go handleVoice(vc.OpusRecv, newSpeaker, closedFiles)

	select {
	case <-quit:
	case <-timeout.C:
	case <-disconnected:
	}
}

func handleVoice(
	packets <-chan *dgo.Packet,
	newSpeaker <-chan *speaker,
	closedFiles chan<- struct{},
) {
	// Consider checking consent status here instead of in the
	// VoiceSpeakingUpdate handler so that toggling consent during a
	// conversation will determine whether the packets are recorded or not.

	speakers := make(map[uint32]*speaker)
loop:
	for p := range packets {
		spk, ok := speakers[p.SSRC]
		for !ok {
			if s := <-newSpeaker; s != nil {
				speakers[s.ssrc] = s
				spk, ok = speakers[p.SSRC]
			} else {
				break loop
			}
		}
		if spk.file == nil {
			// Ignore non-consenting users
			continue
		}
		rtpPacket := createRTPPacket(p)
		if err := spk.file.WriteRTP(rtpPacket); err != nil {
			// TODO Consider marking the file as corrupt
			log.Printf("failed to write RTP data for %v: %v\n", p.SSRC, err)
		}
	}

	defer func() { closedFiles <- struct{}{} }()
	for _, s := range speakers {
		if s.file != nil {
			if err := s.file.Close(); err != nil {
				log.Println("failed to close file: ", err)
			}
			dbEndAudio(s.audioId)
		}
	}
}

func createRTPPacket(p *dgo.Packet) *rtp.Packet {
	return &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    0x78,
			SequenceNumber: p.Sequence,
			Timestamp:      p.Timestamp,
			SSRC:           p.SSRC,
		},
		Payload: p.Opus,
	}
}
