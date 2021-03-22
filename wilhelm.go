package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type messageHandler = func(*discordgo.Session, *discordgo.MessageCreate, ...string) error

var (
	token    = os.Getenv("TOKEN")
	commands = map[string]messageHandler{
		"consent": consent,
		"witness": witness,
	}
)

func init() {
	if token == "" {
		log.Fatalln("TOKEN environment variable not specified")
	}
}

func onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if prefix := botPrefix(m.GuildID); strings.HasPrefix(m.Content, prefix) {
		rawcmd := m.Content[len(prefix):]
		fields := strings.Fields(rawcmd)
		if cmdFunc, ok := commands[fields[0]]; ok {
			if err := cmdFunc(s, m, fields[1:]...); err != nil {
				s.ChannelMessageSendReply(m.ChannelID, err.Error(), m.Reference())
			}
		} else {
			log.Printf("Invalid command '%s' from %s\n", rawcmd, m.Author)
			s.ChannelMessageSendReply(
				m.ChannelID,
				fmt.Sprintf("Command not recognized: '%s'", fields[0]),
				m.Reference(),
			)
		}
	}
}

func main() {
	defer db.Close()

	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln(err)
	}
	discord.StateEnabled = true
	discord.Identify.Intents = discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessages
	if err := discord.Open(); err != nil {
		panic(err)
	}
	defer discord.Close()

	stopHandling := discord.AddHandler(onMessage)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	log.Println(<-quit)
	stopHandling()
	listening <- struct{}{} // Wait for disconnect from voice
}
