package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
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

type Speaker struct {
	uid  string
	ssrc uint32
	file media.Writer
}

func listen(s *dgo.Session, vc *dgo.VoiceConnection) {
	var (
		timeout      = time.NewTimer(10 * time.Minute)
		quit         = make(chan os.Signal, 1)
		disconnected = make(chan struct{})
		closedFiles  = make(chan struct{})
		newSpeaker   = make(chan *Speaker)
	)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	stopHandling := s.AddHandler(func(s *dgo.Session, m *dgo.VoiceStateUpdate) {
		if m.UserID == vc.UserID {
			if m.ChannelID == "" {
				log.Printf("we have been forcibly disconnected from '%s'\n",
					m.BeforeUpdate.ChannelID)
				disconnected <- struct{}{}
			} else {
				// TODO What should happen when we are moved to another channel?
				log.Printf("we have moved to %s", m.ChannelID)
			}
		}
	})
	vc.AddHandler(func(vc *dgo.VoiceConnection, vs *dgo.VoiceSpeakingUpdate) {
		// Experimentally, vs.Speaking is never false. Why?
		if vs.Speaking {
			// TODO network file storage
			file, err := oggwriter.New(fmt.Sprintf("%s/%s.ogg", mediaRoot, vs.UserID),
				48000, 2)
			if err != nil {
				log.Panicln(err)
			}
			newSpeaker <- &Speaker{vs.UserID, uint32(vs.SSRC), file}
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
	newSpeaker <-chan *Speaker,
	closedFiles chan<- struct{},
) {
	speakers := make(map[uint32]*Speaker)
loop:
	for p := range packets {
		s, ok := speakers[p.SSRC]
		for !ok {
			if spk := <-newSpeaker; spk != nil {
				speakers[spk.ssrc] = spk
				s, ok = speakers[p.SSRC]
			} else {
				break loop
			}
		}
		rtpPacket := createRTPPacket(p)
		if err := s.file.WriteRTP(rtpPacket); err != nil {
			// TODO Consider marking the file as corrupt
			log.Printf("failed to write RTP data for %v: %v\n", p.SSRC, err)
		}
	}
	for _, s := range speakers {
		if err := s.file.Close(); err != nil {
			log.Println("failed to close file: ", err)
		}
	}
	closedFiles <- struct{}{}
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
