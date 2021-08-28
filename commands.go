package main

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	dgo "github.com/bwmarrin/discordgo"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

var (
	ErrWilhelmBusy = errors.New("wilhelm is busy right now, try again later")
)

func cmdConsent(s *dgo.Session, m *dgo.MessageCreate, _ []string) error {
	msg := "I didn't want to listen to you anyway 😔"
	if dbToggleConsent(m.Author.ID) {
		msg = "I will be your witness"
	}
	s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
	return nil
}

func cmdCheckConsent(s *dgo.Session, m *dgo.MessageCreate, _ []string) error {
	consent := dbGetConsent(m.Author.ID)
	msg := fmt.Sprint("Consenting:", consent)
	s.ChannelMessageSendReply(m.ChannelID, msg, m.Reference())
	return nil
}

func cmdWitness(s *dgo.Session, m *dgo.MessageCreate, args []string) (ret error) {
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

	vc := joinVoiceFromMessage(s, m)
	go listen(s, vc, duration, func (packets <-chan *dgo.Packet, spkUpdate <-chan *speakerId, done chan<- bool) {
		convId := dbCreateConversation(vc.GuildID)
		defer func() {
			dbEndConversation(convId)
			done <- true
		}()

		ignore := make(map[uint32]bool)
		speakers := make(map[uint32]speaker)
		for p := range packets {
			if ignore[p.SSRC] {
				continue
			}
			spk, ok := speakers[p.SSRC]
			if !ok {
				fileUrl := constructUri(convId, p.SSRC)
				audioId := dbCreateAudio(convId, fileUrl.String())
				writer, err := oggwriter.New(fileUrl.Path, 48000, 2)
				if err != nil {
					log.Panicln("failed to open ogg writer for '", fileUrl.Path, "':", err)
				}
				spk = speaker{p.SSRC, writer, audioId}
				speakers[spk.ssrc] = spk
			}
			select {
			case up := <-spkUpdate:
				if spk := speakers[up.ssrc]; up.consent {
					dbAudioSetUserID(spk.audioId, up.uid)
				} else {
					dbPurgeAudioData(spk.audioId)
					ignore[spk.ssrc] = true
					delete(speakers, spk.ssrc)
					continue
				}
			default:
			}
			rtpPacket := createRTPPacket(p)
			if err := spk.file.WriteRTP(rtpPacket); err != nil {
				log.Printf("failed to write some RTP data for %v: %v\n", p.SSRC, err)
			}
		}

		for _, s := range speakers {
			if err := s.file.Close(); err != nil {
				log.Println("failed to close file: ", err)
			}
			dbEndAudio(s.audioId)
		}
	})

	return nil
}

func cmdAdjourn(_ *dgo.Session, _ *dgo.MessageCreate, _ []string) error {
	log.Println("session adjourned")
	voiceDisconnect <- true
	return nil
}
