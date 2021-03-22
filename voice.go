package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// Semaphore signalling when we are listening
	listening = make(chan struct{}, 1)
)

func listen(s *discordgo.Session, vc *discordgo.VoiceConnection) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	timeout := time.NewTimer(10 * time.Minute)
	disconnected := make(chan struct{})
	stopHandling := s.AddHandler(func(s *discordgo.Session, m *discordgo.VoiceStateUpdate) {
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
