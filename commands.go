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
		if recover() != nil {
			<-listening
			ret = fmt.Errorf("having trouble connecting. try again later")
		}
	}()

	select {
	case listening <- struct{}{}:
	default:
		return ErrWilhelmBusy
	}

	vs, err := s.State.VoiceState(m.GuildID, m.Author.ID)
	if err != nil {
		log.Panicln("error getting voice state: ", err)
	}
	vc, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, false)
	if err != nil {
		log.Panicln("error joining voice channel: ", err)
	}

	duration := 10 * time.Minute
	if len(args) >= 1 {
		if arg, err := strconv.Atoi(args[0]); err != nil {
			log.Println("failed parsing duration '", args[0], "': ", err)
		} else if newtime := time.Duration(arg) * time.Second; newtime > 1*time.Hour {
			duration = 1 * time.Hour
		} else if newtime < 1*time.Second {
			duration = 1 * time.Second
		} else {
			duration = newtime
		}
	}
	convId := dbCreateConversation(vc.GuildID)
	go listen(s, vc, convId, duration)

	return nil
}
