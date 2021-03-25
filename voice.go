package main

import (
	"fmt"
	"log"
	"net/url"
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
	listening = make(chan bool, 1)
)

type speaker struct {
	ssrc    uint32
	file    media.Writer
	audioId int64
}

type speakerId struct {
	uid     string
	ssrc    uint32
	consent bool
}

func listen(s *dgo.Session, vc *dgo.VoiceConnection, convId int64, duration time.Duration) {
	var (
		timeout      = time.NewTimer(duration)
		quit         = make(chan os.Signal, 1)
		disconnected = make(chan bool)
		voiceDone    = make(chan bool)
		speakerIds   = make(chan *speakerId)
	)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	stopHandling := s.AddHandler(func(s *dgo.Session, m *dgo.VoiceStateUpdate) {
		if m.UserID == vc.UserID {
			if m.ChannelID == "" {
				log.Printf("forcibly disconnected from '%s'\n",
					m.BeforeUpdate.ChannelID)
				disconnected <- true
			} else {
				// TODO What should happen when we are moved to another channel?
				log.Printf("moved to %s", m.ChannelID)
			}
		}
	})
	vc.AddHandler(func(vc *dgo.VoiceConnection, vs *dgo.VoiceSpeakingUpdate) {
		log.Println("speaking update:", *vs)
		// Experimentally, vs.Speaking is never false. Why?
		if !vs.Speaking {
			return
		}
		speakerIds <- &speakerId{vs.UserID, uint32(vs.SSRC), dbIsConsenting(vs.UserID)}
	})

	defer func() {
		stopHandling()
		if err := vc.Disconnect(); err != nil {
			log.Println(err)
		}
		signal.Stop(quit)
		timeout.Stop()
		close(vc.OpusRecv)
		close(speakerIds)
		dbEndConversation(convId)
		<-voiceDone
		<-listening
	}()

	go handleVoice(vc.OpusRecv, speakerIds, voiceDone, convId)

	select {
	case <-quit:
	case <-timeout.C:
	case <-disconnected:
	}
}

func handleVoice(
	packets <-chan *dgo.Packet,
	spkUpdate <-chan *speakerId,
	done chan<- bool,
	convId int64,
) {
	defer func() {
		done <- true
	}()

	ignore := make(map[uint32]bool)
	speakers := make(map[uint32]speaker)
	for p := range packets {
		if ignore[p.SSRC] {
			continue
		}
		spk, ok := speakers[p.SSRC]
		if !ok {
			fileUrl := constructUri(convId, p.SSRC)
			audioId := dbCreateAudio(convId, fileUrl.String())
			writer, err := oggwriter.New(fileUrl.Path, 48000, 2)
			if err != nil {
				log.Panicln("failed to open ogg writer for '", fileUrl.Path, "':", err)
			}
			spk = speaker{p.SSRC, writer, audioId}
			speakers[spk.ssrc] = spk
		}
		select {
		case up := <-spkUpdate:
			if spk := speakers[up.ssrc]; up.consent {
				dbAudioSetUserID(spk.audioId, up.uid)
			} else {
				dbPurgeAudioData(spk.audioId)
				ignore[spk.ssrc] = true
				delete(speakers, spk.ssrc)
				continue
			}
		default:
		}
		rtpPacket := createRTPPacket(p)
		if err := spk.file.WriteRTP(rtpPacket); err != nil {
			log.Printf("failed to write some RTP data for %v: %v\n", p.SSRC, err)
		}
	}

	for _, s := range speakers {
		if err := s.file.Close(); err != nil {
			log.Println("failed to close file: ", err)
		}
		dbEndAudio(s.audioId) // Consider not tracked when the audio ends because it is similar
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

func constructUri(convId int64, ssrc uint32) *url.URL {
	url := *mediaRoot
	url.Path = path.Join(url.Path, fmt.Sprintf("%v_%v.ogg", convId, ssrc))
	return &url
}
