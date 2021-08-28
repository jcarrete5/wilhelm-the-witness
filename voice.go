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
)

var (
	// Semaphore signalling when we are listening
	listening       = make(chan bool, 1) // TODO: Might want to make this per-guild
	voiceDisconnect = make(chan bool)
)

type listenHandler func(packets <-chan *dgo.Packet, spkUpdate <-chan *speakerId, done chan<- bool)

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

func joinVoiceFromMessage(s *dgo.Session, m *dgo.MessageCreate) (vc *dgo.VoiceConnection) {
	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil {
		log.Panicln("error getting voice state:", err)
	}
	vc, err = s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, false)
	defer func() {
		if msg := recover(); msg != nil {
			if err := vc.Disconnect(); err != nil {
				log.Println("failed disconnecting from voice:", err)
			}
			panic(msg)
		}
	}()
	if err != nil {
		log.Panicln("error joining voice channel:", err)
	}
	return
}

func listen(s *dgo.Session, vc *dgo.VoiceConnection, duration time.Duration, handler listenHandler) {
	var (
		timeout    = time.NewTimer(duration)
		quit       = make(chan os.Signal, 1)
		voiceDone  = make(chan bool)
		speakerIds = make(chan *speakerId)
	)

	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	stopHandling := s.AddHandler(func(s *dgo.Session, m *dgo.VoiceStateUpdate) {
		if m.UserID == vc.UserID {
			if m.ChannelID == "" {
				log.Printf("forcibly disconnected from '%s'\n",
					m.BeforeUpdate.ChannelID)
				voiceDisconnect <- true
			} else {
				// TODO What should happen when we are moved to another channel?
				log.Printf("moved to %s", m.ChannelID)
			}
		}
	})
	vc.AddHandler(func(vc *dgo.VoiceConnection, vsu *dgo.VoiceSpeakingUpdate) {
		log.Printf("speaking update: %+v\n", *vsu)
		// Experimentally, vsu.Speaking is never false. Why?
		if !vsu.Speaking {
			return
		}
		speakerIds <- &speakerId{vsu.UserID, uint32(vsu.SSRC), dbIsConsenting(vsu.UserID)}
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
		<-voiceDone // Wait for handler to clean up
		<-listening
	}()

	go handler(vc.OpusRecv, speakerIds, voiceDone)

	select {
	case <-quit:
	case <-timeout.C:
	case <-voiceDisconnect:
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
