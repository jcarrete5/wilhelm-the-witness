package main

import (
	"errors"
	"github.com/bwmarrin/discordgo"
	"log"
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
