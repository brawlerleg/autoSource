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
	http.HandleFunc("/api/parts", getParts)
	http.HandleFunc("/api/login", loginHandler)
	http.HandleFunc("/api/logout", logoutHandler)
	http.HandleFunc("/api/check-auth", checkAuthHandler)
	http.HandleFunc("/api/register", registerHandler)
	http.HandleFunc("/api/me", meHandler) // GET — данные текущего юзера

	// ── Защищённые маршруты ───────────────────────────────────
	http.HandleFunc("/api/parts/add", authMiddleware(addPart))

	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
