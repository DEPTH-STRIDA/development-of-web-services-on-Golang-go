package repository

import (
	"sync"
	"taskbot/internal/models"
)

var (
	GlobalMemory *Memory
)

func init() {
	GlobalMemory = NewMemory()
}

type Memory struct {
	sync.RWMutex
	tasks  []models.Task
	users  map[int64]*models.User
	lastID int64
}

func NewMemory() *Memory {
	return &Memory{
		tasks:  make([]models.Task, 0),
		users:  make(map[int64]*models.User),
		lastID: 0,
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
	return user, exists
}

// Create добавляет новую задачу
func (m *Memory) Create(task models.Task) int64 {
	m.Lock()
	defer m.Unlock()

	m.lastID++
	task.ID = m.lastID
	m.tasks = append(m.tasks, task)
	return task.ID
}

// GetByID возвращает задачу по ID
func (m *Memory) GetByID(id int64) (*models.Task, bool) {
	m.RLock()
	defer m.RUnlock()

	for i := range m.tasks {
		if m.tasks[i].ID == id {
			return &m.tasks[i], true
		}
	}
	return nil, false
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
