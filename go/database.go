package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

// hashPassword возвращает hex-строку SHA-256 от пароля.
func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", sum)
}

// initDB открывает файл базы данных и возвращает соединение.
func initDB() *sql.DB {
	database, err := sql.Open("sqlite", "./shop.db")
	if err != nil {
		log.Fatal("initDB — ошибка открытия БД:", err)
	}
	return database
}

// prepareDB создаёт таблицы и засевает данными при первом запуске.
func prepareDB() {
	// ── Таблица запчастей ────────────────────────────────────────
	createParts := `
	CREATE TABLE IF NOT EXISTS parts (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		name    TEXT    NOT NULL,
		article TEXT    NOT NULL,
		price   INTEGER NOT NULL DEFAULT 0
	);`
	if _, err := db.Exec(createParts); err != nil {
		log.Fatal("prepareDB — не удалось создать таблицу parts:", err)
	}

	// ── Таблица пользователей ────────────────────────────────────
	// Добавлены поля email и phone для входа по email/телефону.
	// username оставлен для обратной совместимости и внутреннего использования.
	// email и phone допускают NULL — не у всех пользователей могут быть оба.
	createUsers := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    NOT NULL UNIQUE,
		email         TEXT    UNIQUE,
		phone         TEXT    UNIQUE,
		password_hash TEXT    NOT NULL,
		role          TEXT    NOT NULL DEFAULT 'admin'
	);`
	if _, err := db.Exec(createUsers); err != nil {
		log.Fatal("prepareDB — не удалось создать таблицу users:", err)
	}

	// ── Миграция: добавляем колонки email и phone если их ещё нет ─
	// Нужно при обновлении существующей БД без пересоздания таблицы.
	migrations := []string{
		"ALTER TABLE users ADD COLUMN email TEXT UNIQUE",
		"ALTER TABLE users ADD COLUMN phone TEXT UNIQUE",
	}
	for _, m := range migrations {
		// Игнорируем ошибку — колонка уже существует, это нормально
		db.Exec(m)
	}

	// ── Первый администратор (если таблица users пустая) ─────────
	var userCount int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if userCount == 0 {
		_, err := db.Exec(
			`INSERT INTO users (username, email, phone, password_hash, role)
			 VALUES (?, ?, ?, ?, ?)`,
			"admin",
			"admin@autopartstore.ru", // email для входа
			"+79000000000",           // телефон для входа
			hashPassword("admin123"),
			"admin",
		)
		if err != nil {
			log.Fatal("prepareDB — не удалось создать первого администратора:", err)
		}
		log.Println("prepareDB — создан администратор:")
		log.Println("  email:    admin@autopartstore.ru")
		log.Println("  телефон: +79000000000")
		log.Println("  пароль:  admin123")
	}

	// ── Тестовые запчасти (если таблица parts пустая) ────────────
	var partCount int
	db.QueryRow("SELECT COUNT(*) FROM parts").Scan(&partCount)
	if partCount > 0 {
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
		log.Fatal("prepareDB — ошибка подготовки INSERT:", err)
	}
	defer stmt.Close()

	for _, p := range seed {
		if _, err := stmt.Exec(p.Name, p.Article, p.Price); err != nil {
			log.Println("prepareDB — ошибка вставки тестовых данных:", err)
		}
	}
	log.Println("prepareDB — тестовые данные загружены в БД")
}
