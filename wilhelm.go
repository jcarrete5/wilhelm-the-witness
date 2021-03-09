package main

import (
	"fmt"
	"os"
	"time"
	"github.com/bwmarrin/discordgo"
)

func main() {
	token := os.Getenv("TOKEN")
	discord, err := discordgo.New("Bot " + token)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = discord.Open()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer discord.Close()

	const channelID = "525449096006467620"
	const guildID = "525449096006467614"
	vc, err := discord.ChannelVoiceJoin(guildID, channelID, true, true)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	time.Sleep(10 * time.Second)

	err = vc.Disconnect()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
