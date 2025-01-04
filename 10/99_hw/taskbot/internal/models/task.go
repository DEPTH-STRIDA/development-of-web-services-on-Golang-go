package models

// Task - структура задачи
type Task struct {
	// ID - id задачи
	ID int64
	// Title - название задачи
	Title string
	// Owner - владелец задачи
	Owner *User
	// Assignee - исполнитель задачи
	Assignee *User
}

// User - структура пользователя
type User struct {
	// ID - id пользователя (например телеграм id)
	ID int64
	// Username - имя пользователя (например телеграм username)
	Username string
}
