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

var sessions = map[string]string{} // token → username

// ── Общие типы ────────────────────────────────────────────────

type loginResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

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
// Принимает: { "name": "Иван", "email": "user@example.com", "password": "..." }
// Возвращает: 201 при успехе, 400 при ошибке валидации, 409 если email занят.
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

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Некорректный JSON"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)

	// Валидация
	switch {
	case req.Name == "":
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Укажите ваше имя"})
		return
	case req.Email == "":
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Укажите email"})
		return
	case !strings.Contains(req.Email, "@"):
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Некорректный формат email"})
		return
	case len(req.Password) < 6:
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Пароль должен быть не менее 6 символов"})
		return
	}

	// Проверка уникальности email
	var existingID int
	err := db.QueryRow("SELECT id FROM users WHERE email = ?", req.Email).Scan(&existingID)
	if err == nil {
		writeJSON(w, http.StatusConflict,
			loginResponse{OK: false, Message: "Этот email уже зарегистрирован"})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		log.Println("registerHandler — проверка email:", err)
		writeJSON(w, http.StatusInternalServerError,
			loginResponse{OK: false, Message: "Ошибка базы данных"})
		return
	}

	// Сохранение
	_, err = db.Exec(
		`INSERT INTO users (username, name, email, password_hash, role)
		 VALUES (?, ?, ?, ?, ?)`,
		req.Email, // username = email (уникальный идентификатор)
		req.Name,
		req.Email,
		hashPassword(req.Password),
		"user",
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusConflict,
				loginResponse{OK: false, Message: "Email уже зарегистрирован"})
			return
		}
		log.Println("registerHandler — INSERT:", err)
		writeJSON(w, http.StatusInternalServerError,
			loginResponse{OK: false, Message: "Ошибка сохранения"})
		return
	}

	log.Printf("registerHandler — новый пользователь %q (%s)",
		req.Name, maskIdentifier(req.Email))
	writeJSON(w, http.StatusCreated,
		loginResponse{OK: true, Message: "Аккаунт успешно создан"})
}

// ── loginHandler — POST /api/login ───────────────────────────

type loginRequest struct {
	Identifier string `json:"identifier"`
	Username   string `json:"username"` // устаревшее поле — поддерживается
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
	if identifier == "" {
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Введите email"})
		return
	}
	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Введите пароль"})
		return
	}

	switch classifyIdentifier(identifier) {
	case idEmail:
		log.Printf("loginHandler — вход по email %s", maskIdentifier(identifier))
	default:
		log.Printf("loginHandler — вход по username %s", maskIdentifier(identifier))
	}

	var storedHash, foundUsername string
	err := db.QueryRow(
		`SELECT username, password_hash FROM users
		 WHERE email = ? OR username = ? LIMIT 1`,
		identifier, identifier,
	).Scan(&foundUsername, &storedHash)

	if err != nil || storedHash != hashPassword(req.Password) {
		writeJSON(w, http.StatusUnauthorized,
			loginResponse{OK: false, Message: "Неверный email или пароль"})
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
		Name: cookieName, Value: "",
		Path: "/", Expires: time.Unix(0, 0), MaxAge: -1,
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

// ── meHandler — GET /api/me ───────────────────────────────────
//
// Возвращает данные текущего пользователя по session_token Cookie.
// Ответ: { ok, name, email, role, displayName }
func meHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized,
			loginResponse{OK: false, Message: "Не авторизован"})
		return
	}

	username, ok := sessions[cookie.Value]
	if !ok {
		writeJSON(w, http.StatusUnauthorized,
			loginResponse{OK: false, Message: "Сессия истекла"})
		return
	}

	// Читаем name, email, role из БД по username
	var name, role string
	var emailPtr *string

	err = db.QueryRow(
		"SELECT name, email, role FROM users WHERE username = ?", username,
	).Scan(&name, &emailPtr, &role)

	if err != nil {
		log.Println("meHandler — запрос пользователя:", err)
		writeJSON(w, http.StatusInternalServerError,
			loginResponse{OK: false, Message: "Ошибка базы данных"})
		return
	}

	email := ""
	if emailPtr != nil {
		email = *emailPtr
	}

	// displayName: имя > email > username
	displayName := username
	if name != "" {
		displayName = name
	} else if email != "" {
		displayName = email
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"ok":          true,
		"username":    username,
		"name":        name,
		"email":       email,
		"role":        role,
		"displayName": displayName,
	})
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
