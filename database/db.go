package database

import (
    "database/sql"
    "log"
    _ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() error {
    var err error
    DB, err = sql.Open("sqlite3", "forum.db")
    if err != nil {
        return err
    }

    // Create tables
    _, err = DB.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id TEXT PRIMARY KEY,
            nickname TEXT UNIQUE,
            age INTEGER,
            gender TEXT,
            first_name TEXT,
            last_name TEXT,
            email TEXT UNIQUE,
            password TEXT
        );
        CREATE TABLE IF NOT EXISTS messages (
            id TEXT PRIMARY KEY,
            sender_id TEXT,
            receiver_id TEXT,
            content TEXT,
            created_at DATETIME,
            FOREIGN KEY (sender_id) REFERENCES users(id),
            FOREIGN KEY (receiver_id) REFERENCES users(id)
        );
        CREATE TABLE IF NOT EXISTS posts (
            id TEXT PRIMARY KEY,
            user_id TEXT,
            title TEXT,
            content TEXT,
            categories TEXT,
            created_at DATETIME,
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
        CREATE TABLE IF NOT EXISTS comments (
            id TEXT PRIMARY KEY,
            post_id TEXT,
            user_id TEXT,
            content TEXT,
            created_at DATETIME,
            FOREIGN KEY (post_id) REFERENCES posts(id),
            FOREIGN KEY (user_id) REFERENCES users(id)
        );
    `)
    if err != nil {
        log.Fatal(err)
    }
    
    return DB.Ping()
}