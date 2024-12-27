package models

type Task struct {
	ID       int64
	Title    string
	Owner    *User
	Assignee *User
}

type User struct {
	ID       int64
	Username string
}
