package main

import (
	"fmt"
	"net/http"
)

// Обработчик для корневого пути
func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to the root!")
}

// Обработчик для пути /hello
func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello, World!")
}

func main() {
	// Создаем новый маршрутизатор
	mux := http.NewServeMux()

	// Определяем обработчики как отдельные переменные
	root := http.HandlerFunc(rootHandler)
	hello := http.HandlerFunc(helloHandler)

	// Регистрируем обработчики в маршрутизаторе
	mux.Handle("/", root)
	mux.Handle("/hello", hello)

	// Запускаем сервер
	http.ListenAndServe(":8080", mux)
}
