package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const sessionDuration = 24 * time.Hour
const cookieName = "session_token"

// sessions — хранилище активных сессий в памяти.
var sessions = map[string]string{}

// ── Общие типы ────────────────────────────────────────────────

type loginResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ── Вспомогательные функции ───────────────────────────────────

// writeJSON пишет JSON-ответ с нужным статусом в одну строку.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── classifyIdentifier ────────────────────────────────────────

type idType int

const (
	idUnknown idType = iota
	idEmail
	idPhone
	idUsername
)

func classifyIdentifier(s string) idType {
	if strings.Contains(s, "@") {
		return idEmail
	}
	phoneChars := 0
	for _, r := range s {
		if unicode.IsDigit(r) || r == '+' || r == '-' || r == ' ' || r == '(' || r == ')' {
			phoneChars++
		}
	}
	if phoneChars == len([]rune(s)) && strings.ContainsAny(s, "0123456789") {
		return idPhone
	}
	return idUsername
}

// ── maskIdentifier ────────────────────────────────────────────

func maskIdentifier(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	if strings.Contains(s, "@") {
		parts := strings.SplitN(s, "@", 2)
		name := parts[0]
		if len(name) > 2 {
			name = name[:2] + strings.Repeat("*", len(name)-2)
		}
		return name + "@" + parts[1]
	}
	runes := []rune(s)
	if len(runes) > 8 {
		return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
	}
	return string(runes[:2]) + strings.Repeat("*", len(runes)-2)
}

// ── registerHandler — POST /api/register ─────────────────────
//
// Принимает JSON:
//
//	{
//	  "email":    "user@example.com",
//	  "phone":    "+79001234567",
//	  "password": "secret123"
//	}
//
// Коды ответа:
//
//	201 — пользователь создан
//	400 — не заполнены обязательные поля
//	409 — email или телефон уже заняты
//	500 — ошибка БД
func registerHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// ── Декодирование ─────────────────────────────────────────
	var req struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println("registerHandler — ошибка декодирования JSON:", err)
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Некорректный JSON"})
		return
	}

	// ── Клиентская валидация на сервере ───────────────────────
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Email == "" && req.Phone == "" {
		writeJSON(w, http.StatusBadRequest, loginResponse{
			OK:      false,
			Message: "Укажите email или номер телефона",
		})
		return
	}
	if req.Email != "" && !strings.Contains(req.Email, "@") {
		writeJSON(w, http.StatusBadRequest, loginResponse{
			OK:      false,
			Message: "Некорректный формат email",
		})
		return
	}
	if len(req.Password) < 6 {
		writeJSON(w, http.StatusBadRequest, loginResponse{
			OK:      false,
			Message: "Пароль должен быть не менее 6 символов",
		})
		return
	}

	// ── Проверка уникальности email ───────────────────────────
	if req.Email != "" {
		var existingID int
		err := db.QueryRow(
			"SELECT id FROM users WHERE email = ?", req.Email,
		).Scan(&existingID)

		if err == nil {
			// Запись найдена — email занят
			writeJSON(w, http.StatusConflict, loginResponse{
				OK:      false,
				Message: "Этот email уже зарегистрирован",
			})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("registerHandler — ошибка проверки email:", err)
			writeJSON(w, http.StatusInternalServerError, loginResponse{
				OK: false, Message: "Ошибка базы данных",
			})
			return
		}
	}

	// ── Проверка уникальности телефона ────────────────────────
	if req.Phone != "" {
		var existingID int
		err := db.QueryRow(
			"SELECT id FROM users WHERE phone = ?", req.Phone,
		).Scan(&existingID)

		if err == nil {
			writeJSON(w, http.StatusConflict, loginResponse{
				OK:      false,
				Message: "Этот номер телефона уже зарегистрирован",
			})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("registerHandler — ошибка проверки телефона:", err)
			writeJSON(w, http.StatusInternalServerError, loginResponse{
				OK: false, Message: "Ошибка базы данных",
			})
			return
		}
	}

	// ── Формируем username из email или телефона ──────────────
	// Username нужен для внутреннего использования и обратной совместимости.
	username := req.Email
	if username == "" {
		username = req.Phone
	}

	// ── Сохранение в БД ───────────────────────────────────────
	// email и phone могут быть NULL (передаём nil если пустые)
	var emailVal, phoneVal any
	if req.Email != "" {
		emailVal = req.Email
	}
	if req.Phone != "" {
		phoneVal = req.Phone
	}

	_, err := db.Exec(
		`INSERT INTO users (username, email, phone, password_hash, role)
		 VALUES (?, ?, ?, ?, ?)`,
		username,
		emailVal,
		phoneVal,
		hashPassword(req.Password), // SHA-256, никогда не храним пароль в открытом виде
		"user",                     // новые пользователи — обычные, не админы
	)
	if err != nil {
		// UNIQUE constraint: теоретически могло появиться между двумя запросами
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusConflict, loginResponse{
				OK:      false,
				Message: "Email или телефон уже зарегистрированы",
			})
			return
		}
		log.Println("registerHandler — ошибка INSERT:", err)
		writeJSON(w, http.StatusInternalServerError, loginResponse{
			OK: false, Message: "Ошибка сохранения в базу данных",
		})
		return
	}

	log.Printf("registerHandler — новый пользователь: %s", maskIdentifier(username))
	writeJSON(w, http.StatusCreated,
		loginResponse{OK: true, Message: "Аккаунт успешно создан"})
}

// ── loginHandler — POST /api/login ───────────────────────────

type loginRequest struct {
	Identifier string `json:"identifier"`
	Username   string `json:"username"` // устаревшее, поддерживается
	Password   string `json:"password"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Некорректный JSON"})
		return
	}

	if req.Identifier == "" && req.Username != "" {
		req.Identifier = req.Username
	}

	identifier := strings.TrimSpace(req.Identifier)
	password := req.Password

	if identifier == "" {
		writeJSON(w, http.StatusBadRequest, loginResponse{
			OK: false, Message: "Введите email или номер телефона",
		})
		return
	}
	if password == "" {
		writeJSON(w, http.StatusBadRequest, loginResponse{
			OK: false, Message: "Введите пароль",
		})
		return
	}

	switch classifyIdentifier(identifier) {
	case idEmail:
		log.Printf("loginHandler — вход по email %s", maskIdentifier(identifier))
	case idPhone:
		log.Printf("loginHandler — вход по телефону %s", maskIdentifier(identifier))
	default:
		log.Printf("loginHandler — вход по username %s", maskIdentifier(identifier))
	}

	var storedHash, foundUsername string
	err := db.QueryRow(
		`SELECT username, password_hash
		 FROM   users
		 WHERE  email = ? OR phone = ? OR username = ?
		 LIMIT  1`,
		identifier, identifier, identifier,
	).Scan(&foundUsername, &storedHash)

	if err != nil || storedHash != hashPassword(password) {
		writeJSON(w, http.StatusUnauthorized, loginResponse{
			OK: false, Message: "Неверный email/телефон или пароль",
		})
		return
	}

	token := hashPassword(foundUsername + time.Now().String())
	sessions[token] = foundUsername
	log.Printf("loginHandler — %q вошёл", foundUsername)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(sessionDuration),
	})

	json.NewEncoder(w).Encode(loginResponse{OK: true, Message: "Вход выполнен"})
}

// ── logoutHandler — POST /api/logout ─────────────────────────

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if cookie, err := r.Cookie(cookieName); err == nil {
		delete(sessions, cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    cookieName,
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
		MaxAge:  -1,
	})

	json.NewEncoder(w).Encode(loginResponse{OK: true, Message: "Выход выполнен"})
}

// ── checkAuthHandler — GET /api/check-auth ───────────────────

func checkAuthHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		json.NewEncoder(w).Encode(loginResponse{OK: false})
		return
	}

	_, valid := sessions[cookie.Value]
	json.NewEncoder(w).Encode(loginResponse{OK: valid})
}

// ── isAuthenticated ───────────────────────────────────────────

func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	_, ok := sessions[cookie.Value]
	return ok
}
