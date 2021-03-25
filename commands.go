package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	dgo "github.com/bwmarrin/discordgo"
)

var (
	ErrWilhelmBusy = errors.New("wilhelm is busy right now, try again later")
)

func consent(s *dgo.Session, m *dgo.MessageCreate, _ []string) error {
	msg := "I didn't want to listen to you anyway 😔"
	if dbToggleConsent(m.Author.ID) {
		msg = "I will be your witness"
	}
	s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
	return nil
}

func witness(s *dgo.Session, m *dgo.MessageCreate, args []string) (ret error) {
	defer func() {
		if msg := recover(); msg != nil {
			<-listening
			ret = fmt.Errorf("%v", msg)
		}
	}()

	select {
	case listening <- true:
	default:
		return ErrWilhelmBusy
	}

	duration := 10 * time.Minute
	if len(args) >= 1 {
		if arg, err := strconv.Atoi(args[0]); err != nil {
			log.Panicf("failed parsing duration '%s': %v\n", args[0], err)
		} else if newtime := time.Duration(arg) * time.Second; newtime > 1*time.Hour || newtime < 1*time.Second {
			log.Panicln("duration out of range: [1, 3600]")
		} else {
			duration = newtime
		}
	}

	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil {
		log.Panicln("error getting voice state:", err)
	}
	vc, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, false)
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

	convId := dbCreateConversation(vc.GuildID)
	go listen(s, vc, convId, duration)

	return nil
}

func adjourn(_ *dgo.Session, _ *dgo.MessageCreate, _ []string) error {
	log.Println("session adjourned")
	voiceDisconnect <- true
	return nil
}
