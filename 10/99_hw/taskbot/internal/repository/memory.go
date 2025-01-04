package repository

import (
	"sync"
	"sync/atomic"
	"taskbot/internal/models"
)

// см. exmaple/package.go
// В данном пакете используется смешенный паттерн Singleton

// Экземпляр структуры для взаимодействия с памятью
var (
	GlobalMemory *Memory
)

func init() {
	GlobalMemory = NewMemory()
}

type Memory struct {
	// Примитив синхронизации для работы с памятью
	// Стоит отметить, что существует много других примитивов синхронизации, а также библиотек для работы с
	// потокобезопасным кешем.
	// Есть встроенная библиотека sync.Map, которая является потокобезопасным кешем.
	// Тут нужно досканально изучать варианты и потребности проекта.

	// Мютекс следует размещать над полями, который он защищает.
	sync.RWMutex
	// Последний ID задачи
	lastID int64
	// Хранилище задач
	tasks []models.Task
	// Хранилище пользователей
	users map[int64]*models.User
}

// NewMemory создает новый экземпляр структуры Memory
func NewMemory() *Memory {
	return &Memory{
		RWMutex: sync.RWMutex{},
		tasks:   make([]models.Task, 0),
		users:   make(map[int64]*models.User),
		lastID:  0,
	}
}

// AddUser добавляет или обновляет пользователя
func (m *Memory) AddUser(user *models.User) {
	m.Lock()
	defer m.Unlock()
	m.users[user.ID] = user
}

// GetUser возвращает пользователя по ID
func (m *Memory) GetUser(id int64) (*models.User, bool) {
	m.RLock()
	defer m.RUnlock()
	user, exists := m.users[id]

	// Возвращается ссылка на пользователя, стоит помнить, что пользователь не удаляется и можно вернуть ссылку, а не копию.
	return user, exists
}

// Create добавляет новую задачу
func (m *Memory) Create(task models.Task) int64 {
	m.Lock()
	defer m.Unlock()

	// Используем атомарную операцию для увеличения lastID
	task.ID = atomic.AddInt64(&m.lastID, 1)
	m.tasks = append(m.tasks, task)
	return task.ID
}

// GetByID возвращает копию задачи по ID
func (m *Memory) GetByID(id int64) (models.Task, bool) {
	m.RLock()
	defer m.RUnlock()

	for i := range m.tasks {
		if m.tasks[i].ID == id {
			// Возвращаем копию задачи, чтобы не допустить изменения задачи извне + удаления во время работы с ней.
			return m.tasks[i], true
		}
	}
	return models.Task{}, false
}

// GetAll возвращает все задачи
func (m *Memory) GetAll() []models.Task {
	m.RLock()
	defer m.RUnlock()
	return m.tasks
}

// Delete удаляет задачу по ID
func (m *Memory) Delete(id int64) bool {
	m.Lock()
	defer m.Unlock()

	for i := range m.tasks {
		if m.tasks[i].ID == id {
			// Удаляем элемент
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return true
		}
	}
	return false
}

// Update обновляет существующую задачу
func (m *Memory) Update(task models.Task) bool {
	m.Lock()
	defer m.Unlock()

	for i := range m.tasks {
		if m.tasks[i].ID == task.ID {
			m.tasks[i] = task
			return true
		}
	}
	return false
}

// GetByOwner возвращает задачи определенного владельца
func (m *Memory) GetByOwner(ownerID int64) []models.Task {
	m.RLock()
	defer m.RUnlock()

	result := make([]models.Task, 0)
	for _, task := range m.tasks {
		if task.Owner != nil && task.Owner.ID == ownerID {
			result = append(result, task)
		}
	}
	return result
}

// GetByAssignee возвращает задачи назначенные на пользователя
func (m *Memory) GetByAssignee(assigneeID int64) []models.Task {
	m.RLock()
	defer m.RUnlock()

	result := make([]models.Task, 0)
	for _, task := range m.tasks {
		if task.Assignee != nil && task.Assignee.ID == assigneeID {
			result = append(result, task)
		}
	}
	return result
}
