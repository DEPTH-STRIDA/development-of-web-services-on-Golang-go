package main

import "fmt"

type Logger struct {
}

func (l *Logger) Info(msg string) {
	fmt.Println(msg)
}

func (l *Logger) Error(msg string) {
	fmt.Println(msg)
}

// Пример 2 подходов к использованию
// 1 - экземпляр структуры
// 2 - обращение к пакету

// 1 - экземпляр структуры
var GlobalLogger = &Logger{}

// 2 - обращение к пакету
var logger = &Logger{}

func init() {
	logger = &Logger{}
}

func Info() {
	logger.Info("Info")
}

func Error() {
	logger.Error("Error")
}

// 3 - а также внедрение логгера как поле структуры Dependency Injection
// 4 - Singleton - один экземпляр структуры
// 5 - Factory - создание экземпляра структуры
// 6 - паттерн "Фасад" - объединение интерфейсов в один интерфейс

// Многие из этих паттернов - паттерны ООП.
