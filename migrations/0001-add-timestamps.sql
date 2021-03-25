-- Alter Conversations table
CREATE TEMP TABLE Conversations AS SELECT * FROM main.Conversations;
DROP TABLE IF EXISTS main.Conversations;
CREATE TABLE IF NOT EXISTS main.Conversations (
    ConversationID INTEGER PRIMARY KEY ASC,
    StartedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    EndedAt DATETIME,
    GuildID CHAR(20) REFERENCES Guilds(GuildID) ON UPDATE CASCADE ON DELETE RESTRICT
);
INSERT INTO Conversations SELECT * FROM temp.Conversations;
DROP TABLE temp.Conversations;

-- Alter Audio table
CREATE TEMP TABLE Audio AS SELECT * FROM main.Audio;
DROP TABLE IF EXISTS main.Audio;
CREATE TABLE IF NOT EXISTS Audio (
    AudioID INTEGER PRIMARY KEY ASC,
    StartedAt DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    EndedAt DATETIME,
    URL VARCHAR(100),
    UserID CHAR(20) REFERENCES Users(UserID) ON UPDATE CASCADE ON DELETE SET NULL,
    ConversationID INTEGER REFERENCES Conversations(ConversationID) ON UPDATE CASCADE ON DELETE SET NULL
);
INSERT INTO Audio SELECT * FROM temp.Audio;
DROP TABLE temp.Audio;
