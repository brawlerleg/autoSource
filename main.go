package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

// Структура запчасти (должна совпадать с полями в JS и SQL)
type Part struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Article string `json:"article"`
	Price   int    `json:"price"`
}

var db *sql.DB

func main() {
	var err error
	// 1. Подключаемся к базе
	db, err = sql.Open("sqlite", "./shop.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 2. Создаем таблицу и добавляем тестовый товар, чтобы база не была пустой
	prepareDB()

	// 3. Определяем маршруты (роуты)
	http.HandleFunc("/api/parts", getParts)

	// 4. Запускаем сервер
	log.Println("Сервер запущен на http://localhost:8080/api/parts")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Функция получения данных из БД
func getParts(w http.ResponseWriter, r *http.Request) {
	// Разрешаем фронтенду делать запросы (CORS)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query("SELECT id, name, article, price FROM parts")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var parts []Part
	for rows.Next() {
		var p Part
		if err := rows.Scan(&p.ID, &p.Name, &p.Article, &p.Price); err != nil {
			continue
		}
		parts = append(parts, p)
	}

	// Отправляем данные клиенту в формате JSON
	json.NewEncoder(w).Encode(parts)
}

func prepareDB() {
	query := `
	CREATE TABLE IF NOT EXISTS parts (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, article TEXT, price INTEGER);
	INSERT INTO parts (name, article, price) SELECT 'Фильтр масляный', 'OC90', 850 WHERE NOT EXISTS (SELECT 1 FROM parts WHERE article='OC90');
	`
	db.Exec(query)
}
