package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	dgo "github.com/bwmarrin/discordgo"
)

type messageHandler func(*dgo.Session, *dgo.MessageCreate, []string) error

var (
	token     = os.Getenv("TOKEN")
	mediaRoot *url.URL
	commands  = map[string]messageHandler{
		"consent": cmdConsent,
		"witness": cmdWitness,
		"adjourn": cmdAdjourn,
		"check":   cmdCheckConsent,
	}
)

func init() {
	if token == "" {
		log.Fatalln("TOKEN environment variable not specified")
	}
	if mr := os.Getenv("MEDIA_ROOT"); mr == "" {
		log.Fatalln("MEDIA_ROOT environment variable not specified")
	} else {
		var err error
		mediaRoot, err = url.Parse(mr)
		if err != nil {
			log.Fatalln("failed to parse mediaRoot as URL: ", err)
		}
		switch s := mediaRoot.Scheme; s {
		case "file":
		default:
			log.Fatalf("scheme '%s' not supported for MEDIA_ROOT", s)
		}
	}
}

func onMessage(s *dgo.Session, m *dgo.MessageCreate) {
	if prefix := dbBotPrefix(m.GuildID); strings.HasPrefix(m.Content, prefix) {
		rawcmd := m.Content[len(prefix):]
		fields := strings.Fields(rawcmd)
		if cmdFunc, ok := commands[fields[0]]; ok {
			if err := cmdFunc(s, m, fields[1:]); err != nil {
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
	listening <- true // Wait for voice to cleanup
}
