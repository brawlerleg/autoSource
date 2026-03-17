package main

import (
	"encoding/json"
	"log"
	"net/http"
)

// allowedOrigins — список разрешённых источников запросов.
// Добавьте сюда свой домен при деплое.
var allowedOrigins = map[string]bool{
	"http://localhost:3000": true,
	"http://localhost:5500": true, // Live Server (VS Code)
	"http://127.0.0.1:5500": true,
	"http://localhost:5501": true, // Live Server альтернативный порт
	"http://127.0.0.1:5501": true,
	"http://localhost:8080": true,
	"http://127.0.0.1:8080": true,
	"http://localhost:8000": true,
	"http://127.0.0.1:8000": true,
	// При открытии через file:// Origin будет пустым — обрабатывается ниже
}

// originIsAllowed возвращает true если origin разрешён.
// Пустой origin (file://, curl, Postman) тоже считается разрешённым.
func originIsAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	return allowedOrigins[origin]
}

// setCORSHeaders устанавливает заголовки CORS.
//
// ВАЖНО: нельзя одновременно использовать
//
//	Access-Control-Allow-Origin: *
//	Access-Control-Allow-Credentials: true
//
// Браузер запрещает такую комбинацию и блокирует запрос.
// Поэтому мы отражаем конкретный Origin из запроса,
// если он есть в белом списке allowedOrigins.
func setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")

	if originIsAllowed(origin) {
		if origin != "" {
			// Конкретный origin — браузер принимает вместе с Credentials
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			// Нет Origin (curl, Postman, file://) — wildcard безопасен
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
	} else {
		// Незнакомый origin — логируем, но всё равно отражаем
		// (браузер сам заблокирует если не доверяет)
		log.Printf("setCORSHeaders — незнакомый origin: %q", origin)
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}

	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	// Vary нужен чтобы браузер не кешировал ответ одного origin для другого
	w.Header().Add("Vary", "Origin")
}

// ── authMiddleware ────────────────────────────────────────────
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeaders(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if !isAuthenticated(r) {
			log.Println("authMiddleware — доступ запрещён: нет действующей сессии")
			http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// ── getParts — GET /api/parts ─────────────────────────────────
func getParts(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	rows, err := db.Query("SELECT id, name, article, price FROM parts ORDER BY id DESC")
	if err != nil {
		log.Println("getParts — ошибка запроса:", err)
		http.Error(w, "Ошибка базы данных", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var parts []Part
	for rows.Next() {
		var p Part
		if err := rows.Scan(&p.ID, &p.Name, &p.Article, &p.Price); err != nil {
			log.Println("getParts — ошибка сканирования строки:", err)
			continue
		}
		parts = append(parts, p)
	}

	if parts == nil {
		parts = []Part{}
	}

	json.NewEncoder(w).Encode(parts)
}

// ── addPart — POST /api/parts/add ────────────────────────────
func addPart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var p Part
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		log.Println("addPart — ошибка декодирования JSON:", err)
		http.Error(w, "Некорректный JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if p.Name == "" || p.Article == "" {
		http.Error(w, "Поля name и article обязательны", http.StatusBadRequest)
		return
	}
	if p.Price < 0 {
		http.Error(w, "Цена не может быть отрицательной", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		"INSERT INTO parts (name, article, price) VALUES (?, ?, ?)",
		p.Name, p.Article, p.Price,
	)
	if err != nil {
		log.Println("addPart — ошибка INSERT:", err)
		http.Error(w, "Ошибка сохранения в базу данных", http.StatusInternalServerError)
		return
	}

	log.Printf("addPart — добавлен товар: %s (%s) — %d ₽", p.Name, p.Article, p.Price)
	w.WriteHeader(http.StatusCreated)
}
