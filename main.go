package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	_ "modernc.org/sqlite"
)

// Part — структура запчасти.
// Теги json совпадают с полями, которые ожидает / присылает фронтенд.
type Part struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Article string `json:"article"`
	Price   int    `json:"price"`
}

var db *sql.DB

func main() {
	var err error

	// 1. Подключаемся к SQLite-базе
	db, err = sql.Open("sqlite", "./shop.db")
	if err != nil {
		log.Fatal("Ошибка открытия БД:", err)
	}
	defer db.Close()

	// 2. Создаём таблицу и засеваем тестовые данные
	prepareDB()

	// 3. Маршруты
	http.HandleFunc("/api/parts", getParts)    // GET  — список запчастей
	http.HandleFunc("/api/parts/add", addPart) // POST — добавить запчасть

	// 4. Запуск
	log.Println("Сервер запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// ─── setCORSHeaders ────────────────────────────────────────────────────────
// Вспомогательная функция: устанавливает CORS-заголовки на каждый ответ.
// Вызывается в начале каждого обработчика.
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// ─── getParts ──────────────────────────────────────────────────────────────
// GET /api/parts
// Возвращает все запчасти из базы в виде JSON-массива.
func getParts(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Браузер шлёт OPTIONS перед реальным запросом (preflight) — отвечаем 204
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

	// Если таблица пустая — возвращаем [], а не null
	if parts == nil {
		parts = []Part{}
	}

	json.NewEncoder(w).Encode(parts)
}

// ─── addPart ───────────────────────────────────────────────────────────────
// POST /api/parts/add
// Принимает JSON вида {"name":"...","article":"...","price":1000},
// вставляет запись в БД и возвращает 201 Created.
func addPart(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	// Preflight OPTIONS
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Декодируем тело запроса
	var p Part
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // лишние поля — ошибка
	if err := decoder.Decode(&p); err != nil {
		log.Println("addPart — ошибка декодирования JSON:", err)
		http.Error(w, "Некорректный JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Простая валидация обязательных полей
	if p.Name == "" || p.Article == "" {
		http.Error(w, "Поля name и article обязательны", http.StatusBadRequest)
		return
	}
	if p.Price < 0 {
		http.Error(w, "Цена не может быть отрицательной", http.StatusBadRequest)
		return
	}

	// Вставка в БД
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
	w.WriteHeader(http.StatusCreated) // 201
}

// ─── prepareDB ─────────────────────────────────────────────────────────────
// Создаёт таблицу и добавляет несколько тестовых товаров при первом запуске.
func prepareDB() {
	createTable := `
	CREATE TABLE IF NOT EXISTS parts (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		name    TEXT    NOT NULL,
		article TEXT    NOT NULL,
		price   INTEGER NOT NULL DEFAULT 0
	);`

	if _, err := db.Exec(createTable); err != nil {
		log.Fatal("prepareDB — не удалось создать таблицу:", err)
	}

	// Тестовые товары — вставляем только если таблица пустая
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

	var count int
	db.QueryRow("SELECT COUNT(*) FROM parts").Scan(&count)
	if count > 0 {
		return // данные уже есть — не дублируем
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
