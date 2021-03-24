package main

import (
	"database/sql"
	"errors"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	db *sql.DB
)

func init() {
	var err error
	db, err = sql.Open("sqlite3", "file:wilhelm.db?_fk=on&mode=rw")
	if err != nil {
		log.Fatalln(err)
	}
}

func dbBotPrefix(gid string) (prefix string) {
	row := db.QueryRow("SELECT Prefix FROM Guilds WHERE GuildID = ?;", gid)
	if err := row.Scan(&prefix); errors.Is(err, sql.ErrNoRows) {
		if _, err := db.Exec("INSERT INTO Guilds(GuildID) VALUES (?);", gid); err != nil {
			log.Panicln(err)
		}
		prefix = dbBotPrefix(gid)
	} else if err != nil {
		log.Panicln(err)
	}
	return
}

func dbToggleConsent(uid string) (status bool) {
	query := `
	INSERT OR IGNORE INTO Users(UserID) VALUES (?);
	UPDATE Users SET Consent = Consent != TRUE WHERE UserID = ?;`

	if _, err := db.Exec(query, uid, uid); err != nil {
		log.Panicln(err)
	}
	row := db.QueryRow("SELECT Consent FROM Users WHERE UserID = ?;", uid)
	if err := row.Scan(&status); err != nil {
		log.Panicln(err)
	}
	return
}

func dbIsConsenting(uid string) (consent bool) {
	row := db.QueryRow("SELECT Consent FROM Users WHERE UserID = ?;", uid)
	if err := row.Scan(&consent); errors.Is(err, sql.ErrNoRows) {
		if _, err := db.Exec("INSERT INTO Users(UserID) VALUES (?);", uid); err != nil {
			log.Panicln(err)
		}
		consent = dbIsConsenting(uid)
	} else if err != nil {
		log.Panicln(err)
	}
	return
}

func dbCreateConversation(gid string) (id int64) {
	res, err := db.Exec("INSERT INTO Conversations(GuildID) VALUES (?);", gid)
	if err != nil {
		log.Panicln("failed to insert new conversation: ", err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		log.Panicln("failed to get conversation id: ", err)
	}
	return
}

func dbCreateAudio(uid string, convId int64, uri string) (id int64) {
	res, err := db.Exec("INSERT INTO Audio(UserID, ConversationID, URI) VALUES (?, ?, ?);",
		uid, convId, uri)
	if err != nil {
		log.Panicln("failed to insert new audio: ", err)
	}
	id, err = res.LastInsertId()
	if err != nil {
		log.Panicln("failed to get audio id: ", err)
	}
	return
}

func dbEndAudio(audioId int64) {
	t := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec("UPDATE Audio SET EndedAt = ? WHERE AudioID = ?;", t, audioId)
	if err != nil {
		log.Panicln("failed to set EndedAt for audio '", audioId, "': ", err)
	}
}

func dbEndConversation(convId int64) {
	t := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec("UPDATE Conversations SET EndedAt = ? WHERE ConversationID = ?;",
		t, convId)
	if err != nil {
		log.Panicln("failed to set EndedAt for conversation '", convId, "': ", err)
	}
}
