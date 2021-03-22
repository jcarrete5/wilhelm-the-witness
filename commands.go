package main

import (
	"github.com/bwmarrin/discordgo"
)

func consent(s *discordgo.Session, m *discordgo.MessageCreate, _ ...string) {
	msg := "I didn't want to listen to you anyway 😔"
	if toggleConsent(m.Author.ID) {
		msg = "I will be your witness"
	}
	s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
}
