package main

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	listening = make(chan struct{}, 1)
)

var (
	ErrWilhelmBusy = errors.New("wilhelm is busy right now, try again later")
)

func consent(s *discordgo.Session, m *discordgo.MessageCreate, _ ...string) error {
	msg := "I didn't want to listen to you anyway 😔"
	if toggleConsent(m.Author.ID) {
		msg = "I will be your witness"
	}
	s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
	return nil
}

func witness(s *discordgo.Session, m *discordgo.MessageCreate, args ...string) error {
	defer func() {
		if recover() != nil {
			<-listening
		}
	}()

	select {
	case listening <- struct{}{}:
	default:
		return ErrWilhelmBusy
	}

	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil {
		log.Panicln(err)
	}
	vc, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, false)
	if err != nil {
		log.Panicln(err)
	}

	go listen(s, vc)

	return nil
}

func listen(s *discordgo.Session, vc *discordgo.VoiceConnection) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	timeout := time.NewTimer(10 * time.Minute)
	disconnected := make(chan struct{})
	stopHandling := s.AddHandler(func(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
		if m.UserID == vc.UserID && m.ChannelID == "" {
			log.Printf("we have been forcibly disconnected from '%s'\n", m.ChannelID)
			disconnected <- struct{}{}
		}
	})

	defer func() {
		stopHandling()
		if err := vc.Disconnect(); err != nil {
			log.Println(err)
		}
		signal.Stop(quit)
		timeout.Stop()
		<-listening
	}()

	// TODO handle audio here

	select {
	case <-quit:
	case <-timeout.C:
	case <-disconnected:
	}
}
