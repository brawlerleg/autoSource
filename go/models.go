package main

// Part — структура одной записи о запчасти.
type Part struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Article string `json:"article"`
	Price   int    `json:"price"`
}

// User — структура пользователя.
// Используется в meHandler для возврата данных клиенту.
type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"` // имя > email > username
}
