package main

import (
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"os/signal"
)

var (
	token   = os.Getenv("TOKEN")
	discord *discordgo.Session
	quit    = make(chan os.Signal)
)

func init() {
	if token == "" {
		log.Fatalln("TOKEN environment variable not specified")
	}
	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln(err)
	}
	discord = s
	signal.Notify(quit, os.Interrupt)
}

func onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	log.Println(m)
}

func main() {
	discord.Identify.Intents = discordgo.IntentsGuildVoiceStates |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsGuildMessages
	if err := discord.Open(); err != nil {
		panic(err)
	}
	defer discord.Close()

	discord.AddHandler(onMessage)

	log.Println(<-quit)
}
