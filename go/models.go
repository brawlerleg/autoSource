package main

// Part — структура одной записи о запчасти.
// Теги json совпадают с полями, которые ожидает / присылает фронтенд.
type Part struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Article string `json:"article"`
	Price   int    `json:"price"`
}
