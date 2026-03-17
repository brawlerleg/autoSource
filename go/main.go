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

	// ── Публичные маршруты ────────────────────────────────────
	http.HandleFunc("/api/parts", getParts)              // GET  — каталог
	http.HandleFunc("/api/login", loginHandler)          // POST — вход
	http.HandleFunc("/api/logout", logoutHandler)        // POST — выход
	http.HandleFunc("/api/check-auth", checkAuthHandler) // GET  — проверка сессии
	http.HandleFunc("/api/register", registerHandler)    // POST — регистрация

	// ── Защищённые маршруты (только авторизованные) ───────────
	http.HandleFunc("/api/parts/add", authMiddleware(addPart)) // POST — добавить запчасть

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
