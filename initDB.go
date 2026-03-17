package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3" // Драйвер для SQLite
)

func initDB() *sql.DB {
	// Открываем (или создаем) файл базы данных
	db, err := sql.Open("sqlite3", "./shop.db")
	if err != nil {
		log.Fatal(err)
	}

	// Создаем таблицу, если её еще нет
	query := `
	CREATE TABLE IF NOT EXISTS parts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		article TEXT,
		price INTEGER
	);`

	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}

	return db
}
