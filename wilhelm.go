package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type messageHandler = func(*discordgo.Session, *discordgo.MessageCreate, ...string)

var (
	token    = os.Getenv("TOKEN")
	quit     = make(chan os.Signal)
	commands = map[string]messageHandler{
		"consent": consent,
	}
)

func init() {
	if token == "" {
		log.Fatalln("TOKEN environment variable not specified")
	}
	signal.Notify(quit, os.Interrupt)
}

func onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if prefix := botPrefix(m.GuildID); strings.HasPrefix(m.Content, prefix) {
		rawcmd := m.Content[len(prefix):]
		fields := strings.Fields(rawcmd)
		if cmdFunc, ok := commands[fields[0]]; ok {
			cmdFunc(s, m, fields[1:]...)
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
	discord.Identify.Intents = discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsGuildMessages
	if err := discord.Open(); err != nil {
		panic(err)
	}
	defer discord.Close()

	discord.AddHandler(onMessage)

	log.Println(<-quit)
}
