package main

import (
	"database/sql"
	"log"
	"net/http"
)

var db *sql.DB

func main() {
	db = initDB()
	defer db.Close()

	prepareDB()

	// Фоновая очистка истёкших сессий раз в час
	go cleanExpiredSessions()

	// ── API маршруты ──────────────────────────────────────────
	http.HandleFunc("/api/parts", getParts)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/check-auth", checkAuthHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/me", meHandler)
	http.HandleFunc("/api/parts/add", authMiddleware(addPart))

	// ── Статика фронтенда ─────────────────────────────────────
	fs := http.FileServer(http.Dir("../frontend"))
	http.Handle("/", fs)

	log.Println("Сервер запущен:")
	log.Println("  Сайт: http://localhost:8080")
	log.Println("  API:  http://localhost:8080/api/...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
