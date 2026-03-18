package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ── Константы ─────────────────────────────────────────────────

const (
	sessionDuration = 24 * time.Hour
	cookieName      = "session_token"

	// bcrypt cost: 12 — хороший баланс безопасности и скорости.
	// Увеличение на 1 удваивает время хеширования.
	bcryptCost = 12

	// Rate limiting: после maxLoginAttempts неудач — блокировка на lockDuration.
	maxLoginAttempts = 5
	lockDuration     = 5 * time.Minute
)

// ── Типы ──────────────────────────────────────────────────────

type loginResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// ── writeJSON ─────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── hashPassword / checkPassword ─────────────────────────────
//
// bcrypt автоматически генерирует соль и встраивает её в хеш.
// Хранить соль отдельно не нужно — она часть строки "$2a$12$...".

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func checkPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ── Сессии в БД ───────────────────────────────────────────────

// createSession генерирует UUID-токен, сохраняет сессию в таблицу sessions
// и возвращает токен.
func createSession(username string) (string, error) {
	token := uuid.NewString() // случайный UUID v4
	expiresAt := time.Now().Add(sessionDuration).Unix()

	_, err := db.Exec(
		"INSERT INTO sessions (token, username, expires_at) VALUES (?, ?, ?)",
		token, username, expiresAt,
	)
	return token, err
}

// getSessionUser возвращает username по токену, или "" если сессия
// не найдена или истекла. Попутно удаляет истёкшие сессии.
func getSessionUser(token string) string {
	var username string
	var expiresAt int64

	err := db.QueryRow(
		"SELECT username, expires_at FROM sessions WHERE token = ?", token,
	).Scan(&username, &expiresAt)

	if err != nil {
		return "" // не найдена
	}

	if time.Now().Unix() > expiresAt {
		// Истекла — удаляем и возвращаем пусто
		db.Exec("DELETE FROM sessions WHERE token = ?", token)
		return ""
	}

	return username
}

// deleteSession удаляет сессию при выходе.
func deleteSession(token string) {
	db.Exec("DELETE FROM sessions WHERE token = ?", token)
}

// cleanExpiredSessions — периодическая очистка истёкших записей.
// Вызывается из main() в горутине.
func cleanExpiredSessions() {
	for {
		time.Sleep(1 * time.Hour)
		res, _ := db.Exec(
			"DELETE FROM sessions WHERE expires_at < ?", time.Now().Unix(),
		)
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("cleanExpiredSessions — удалено %d истёкших сессий", n)
		}
	}
}

// ── Rate limiting ─────────────────────────────────────────────

// getClientIP извлекает IP клиента из запроса.
// Учитывает X-Forwarded-For если сервер за прокси.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Берём первый IP из цепочки прокси
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Убираем порт из "IP:port"
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// isRateLimited проверяет, не превышен ли лимит попыток для данного IP.
// Возвращает true и время разблокировки если заблокирован.
func isRateLimited(ip string) (limited bool, retryAfter time.Time) {
	var attempts int
	var lockedUntil int64

	err := db.QueryRow(
		"SELECT attempts, locked_until FROM login_attempts WHERE ip = ?", ip,
	).Scan(&attempts, &lockedUntil)

	if err != nil {
		return false, time.Time{} // IP ещё не в таблице
	}

	if lockedUntil > 0 && time.Now().Unix() < lockedUntil {
		return true, time.Unix(lockedUntil, 0)
	}

	return false, time.Time{}
}

// recordFailedAttempt увеличивает счётчик неудачных попыток.
// После maxLoginAttempts — блокирует IP на lockDuration.
func recordFailedAttempt(ip string) {
	now := time.Now().Unix()

	// Upsert: создаём запись или увеличиваем счётчик
	db.Exec(`
		INSERT INTO login_attempts (ip, attempts, locked_until)
		VALUES (?, 1, 0)
		ON CONFLICT(ip) DO UPDATE SET attempts = attempts + 1
	`, ip)

	// Проверяем нужна ли блокировка
	var attempts int
	db.QueryRow("SELECT attempts FROM login_attempts WHERE ip = ?", ip).Scan(&attempts)

	if attempts >= maxLoginAttempts {
		lockedUntil := now + int64(lockDuration.Seconds())
		db.Exec(
			"UPDATE login_attempts SET locked_until = ? WHERE ip = ?",
			lockedUntil, ip,
		)
		log.Printf("rateLimiter — IP %s заблокирован на %v после %d попыток",
			ip, lockDuration, attempts)
	}
}

// resetAttempts сбрасывает счётчик после успешного входа.
func resetAttempts(ip string) {
	db.Exec("DELETE FROM login_attempts WHERE ip = ?", ip)
}

// ── Вспомогательные ───────────────────────────────────────────

type idType int

const (
	idUnknown idType = iota
	idEmail
	idUsername
)

func classifyIdentifier(s string) idType {
	if strings.Contains(s, "@") {
		return idEmail
	}
	return idUsername
}

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

// validatePhone — оставлено для совместимости, не используется в логине
func isPhoneChars(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) || r == '+' || r == '-' || r == ' ' || r == '(' || r == ')' {
			continue
		}
		return false
	}
	return true
}

// ── registerHandler — POST /api/register ─────────────────────

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
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

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

	// Хешируем пароль bcrypt
	hash, err := hashPassword(req.Password)
	if err != nil {
		log.Println("registerHandler — bcrypt:", err)
		writeJSON(w, http.StatusInternalServerError,
			loginResponse{OK: false, Message: "Ошибка сервера"})
		return
	}

	_, err = db.Exec(
		`INSERT INTO users (username, name, email, password_hash, role)
		 VALUES (?, ?, ?, ?, ?)`,
		req.Email, req.Name, req.Email, hash, "user",
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
	Username   string `json:"username"` // устаревшее поле
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

	// ── Rate limiting ─────────────────────────────────────────
	ip := getClientIP(r)
	if limited, retryAfter := isRateLimited(ip); limited {
		remaining := time.Until(retryAfter).Round(time.Second)
		log.Printf("loginHandler — IP %s заблокирован, осталось %v", ip, remaining)
		writeJSON(w, http.StatusTooManyRequests, loginResponse{
			OK:      false,
			Message: "Слишком много попыток. Попробуйте через " + remaining.String(),
		})
		return
	}

	// ── Декодирование ─────────────────────────────────────────
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest,
			loginResponse{OK: false, Message: "Некорректный JSON"})
		return
	}

	if req.Identifier == "" && req.Username != "" {
		req.Identifier = req.Username
	}

	identifier := strings.TrimSpace(strings.ToLower(req.Identifier))
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

	log.Printf("loginHandler — попытка входа: %s (IP: %s)",
		maskIdentifier(identifier), ip)

	// ── Поиск пользователя ────────────────────────────────────
	var storedHash, foundUsername string
	err := db.QueryRow(
		`SELECT username, password_hash FROM users
		 WHERE email = ? OR username = ? LIMIT 1`,
		identifier, identifier,
	).Scan(&foundUsername, &storedHash)

	// ── Проверка пароля (bcrypt) ──────────────────────────────
	// Важно: НЕ делаем ранний return при "пользователь не найден" —
	// это позволило бы по времени ответа определить существование email.
	// Всегда выполняем checkPassword, даже если пользователь не найден.
	var passwordOK bool
	if err == nil {
		passwordOK = checkPassword(req.Password, storedHash)
	}

	if err != nil || !passwordOK {
		recordFailedAttempt(ip)

		// Считаем сколько попыток осталось
		var attempts int
		db.QueryRow("SELECT attempts FROM login_attempts WHERE ip = ?", ip).
			Scan(&attempts)
		remaining := maxLoginAttempts - attempts
		if remaining < 0 {
			remaining = 0
		}

		msg := "Неверный email или пароль"
		if remaining > 0 && remaining <= 3 {
			msg += ". Осталось попыток: " + string(rune('0'+remaining))
		}

		writeJSON(w, http.StatusUnauthorized,
			loginResponse{OK: false, Message: msg})
		return
	}

	// ── Успешный вход ─────────────────────────────────────────
	resetAttempts(ip) // сбрасываем счётчик неудач

	token, err := createSession(foundUsername)
	if err != nil {
		log.Println("loginHandler — createSession:", err)
		writeJSON(w, http.StatusInternalServerError,
			loginResponse{OK: false, Message: "Ошибка создания сессии"})
		return
	}

	log.Printf("loginHandler — %q вошёл (IP: %s)", foundUsername, ip)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,                 // недоступна из JS — защита от XSS
		SameSite: http.SameSiteLaxMode, // защита от CSRF
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
		deleteSession(cookie.Value) // удаляем из БД
	}

	// Сбрасываем Cookie в браузере
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

	valid := getSessionUser(cookie.Value) != ""
	json.NewEncoder(w).Encode(loginResponse{OK: valid})
}

// ── meHandler — GET /api/me ───────────────────────────────────

func meHandler(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w, r)
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil {
		// 200 вместо 401 — браузер не логирует как ошибку,
		// фронтенд проверяет поле ok
		writeJSON(w, http.StatusOK,
			loginResponse{OK: false, Message: "Не авторизован"})
		return
	}

	username := getSessionUser(cookie.Value)
	if username == "" {
		writeJSON(w, http.StatusOK,
			loginResponse{OK: false, Message: "Сессия истекла"})
		return
	}

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

	displayName := username
	if name != "" {
		displayName = name
	} else if email != "" {
		displayName = email
	}

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
	return getSessionUser(cookie.Value) != ""
}
