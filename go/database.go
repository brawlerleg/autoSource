package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", sum)
}

func initDB() *sql.DB {
	database, err := sql.Open("sqlite", "./shop.db")
	if err != nil {
		log.Fatal("initDB — ошибка открытия БД:", err)
	}
	return database
}

func prepareDB() {
	// ── Таблица запчастей ────────────────────────────────────────
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS parts (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		name    TEXT    NOT NULL,
		article TEXT    NOT NULL,
		price   INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		log.Fatal("prepareDB — таблица parts:", err)
	}

	// ── Таблица пользователей ────────────────────────────────────
	// name     — отображаемое имя (Иван Петров)
	// email    — логин и контакт
	// username — внутренний идентификатор (= email при регистрации)
	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    NOT NULL UNIQUE,
		name          TEXT    NOT NULL DEFAULT '',
		email         TEXT    UNIQUE,
		password_hash TEXT    NOT NULL,
		role          TEXT    NOT NULL DEFAULT 'user'
	)`)
	if err != nil {
		log.Fatal("prepareDB — таблица users:", err)
	}

	// ── Миграции (добавляем колонки к старой БД если их нет) ─────
	for _, m := range []string{
		"ALTER TABLE users ADD COLUMN name  TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE users ADD COLUMN email TEXT UNIQUE",
	} {
		db.Exec(m) // ошибка = колонка уже есть — игнорируем
	}

	// ── Первый администратор ──────────────────────────────────────
	var n int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n)
	if n == 0 {
		_, err = db.Exec(
			`INSERT INTO users (username, name, email, password_hash, role)
			 VALUES (?, ?, ?, ?, ?)`,
			"admin",
			"Администратор",
			"admin@autopartstore.ru",
			hashPassword("admin123"),
			"admin",
		)
		if err != nil {
			log.Fatal("prepareDB — создание admin:", err)
		}
		log.Println("prepareDB — создан администратор: admin@autopartstore.ru / admin123")
	}

	// ── Тестовые запчасти ─────────────────────────────────────────
	var p int
	db.QueryRow("SELECT COUNT(*) FROM parts").Scan(&p)
	if p > 0 {
		return
	}

	seed := []Part{
		{0, "Масляный фильтр MANN OC90", "OC90", 890},
		{0, "Тормозные колодки Bosch передние", "BRK-44", 3250},
		{0, "Топливный фильтр Filtron PP905", "SF-201", 1140},
		{0, "Термостат охлаждающей системы", "WT-8", 2600},
		{0, "Свечи зажигания NGK Iridium (к-т 4 шт.)", "SP-3320", 4800},
		{0, "Воздушный фильтр салона угольный", "AB-77", 760},
		{0, "Ремень ГРМ Gates комплект с роликом", "TM-115", 5490},
		{0, "Стойка переднего амортизатора Kayaba", "SH-202", 7200},
	}
	stmt, err := db.Prepare("INSERT INTO parts (name, article, price) VALUES (?, ?, ?)")
	if err != nil {
		log.Fatal("prepareDB — подготовка INSERT:", err)
	}
	defer stmt.Close()
	for _, v := range seed {
		stmt.Exec(v.Name, v.Article, v.Price)
	}
	log.Println("prepareDB — тестовые запчасти загружены")
}
