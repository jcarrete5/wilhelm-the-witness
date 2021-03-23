package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
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

func botPrefix(gid string) (prefix string) {
	row := db.QueryRow("SELECT Prefix FROM Guilds WHERE GuildID = ?;", gid)
	if err := row.Scan(&prefix); err == sql.ErrNoRows {
		if _, err := db.Exec("INSERT INTO Guilds(GuildID) VALUES (?);", gid); err != nil {
			log.Panicln(err)
		}
		prefix = botPrefix(gid)
	} else if err != nil {
		log.Panicln(err)
	}
	return
}

func toggleConsent(uid string) (status bool) {
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

func isConsenting(uid string) (consent bool) {
	row := db.QueryRow("SELECT Consent FROM Users WHERE UserID = ?;", uid)
	if err := row.Scan(&consent); err == sql.ErrNoRows {
		if _, err := db.Exec("INSERT INTO Users(UserID) VALUES (?);", uid); err != nil {
			log.Panicln(err)
		}
		consent = isConsenting(uid)
	} else if err != nil {
		log.Panicln(err)
	}
	return
}
