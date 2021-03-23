package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	dgo "github.com/bwmarrin/discordgo"
)

type messageHandler = func(*dgo.Session, *dgo.MessageCreate, ...string) error

var (
	token     = os.Getenv("TOKEN")
	mediaRoot = os.Getenv("MEDIA_ROOT")
	commands  = map[string]messageHandler{
		"consent": consent,
		"witness": witness,
	}
)

func init() {
	if token == "" {
		log.Fatalln("TOKEN environment variable not specified")
	}
	if mediaRoot == "" {
		log.Fatalln("MEDIA_ROOT environment variable not specified")
	}
	// TODO Should MEDIA_ROOT be a URL?
}

func onMessage(s *dgo.Session, m *dgo.MessageCreate) {
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

	discord, err := dgo.New("Bot " + token)
	if err != nil {
		log.Fatalln(err)
	}
	discord.StateEnabled = true
	discord.Identify.Intents = dgo.IntentsGuildVoiceStates |
		dgo.IntentsGuildMessages
	if err := discord.Open(); err != nil {
		log.Panicln(err)
	}
	defer discord.Close()

	stopHandling := discord.AddHandler(onMessage)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	log.Println(<-quit)
	stopHandling()
	listening <- struct{}{} // Wait for voice to cleanup
}
