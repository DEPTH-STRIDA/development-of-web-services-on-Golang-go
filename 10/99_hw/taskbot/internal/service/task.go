package service

import (
	"fmt"
	"strings"
	"taskbot/internal/models"
	r "taskbot/internal/repository"
)

// HandleTasks возвращает список всех задач
func HandleTasks(userID int64) string {
	tasks := r.GlobalMemory.GetAll()
	if len(tasks) == 0 {
		return "Нет задач"
	}

	var result []string
	for _, task := range tasks {
		if task.Assignee != nil && task.Assignee.ID == userID {
			result = append(result, fmt.Sprintf("%d. %s by @%s\nassignee: я\n/unassign_%d /resolve_%d",
				task.ID, task.Title, task.Owner.Username, task.ID, task.ID))
		} else if task.Assignee != nil {
			result = append(result, fmt.Sprintf("%d. %s by @%s\nassignee: @%s",
				task.ID, task.Title, task.Owner.Username, task.Assignee.Username))
		} else {
			result = append(result, fmt.Sprintf("%d. %s by @%s\n/assign_%d",
				task.ID, task.Title, task.Owner.Username, task.ID))
		}
	}
	return strings.Join(result, "\n\n")
}

// HandleNew создает новую задачу
func HandleNew(title string, owner *models.User) string {
	task := models.Task{
		Title: title,
		Owner: owner,
	}
	id := r.GlobalMemory.Create(task)
	return fmt.Sprintf("Задача %q создана, id=%d", title, id)
}

// HandleAssign назначает задачу на пользователя
func HandleAssign(taskID int64, user *models.User) map[int64]string {
	task, exists := r.GlobalMemory.GetByID(taskID)
	if !exists {
		return map[int64]string{user.ID: "Задача не найдена"}
	}

	prevAssignee := task.Assignee
	task.Assignee = user
	r.GlobalMemory.Update(*task)

	result := map[int64]string{
		user.ID: fmt.Sprintf("Задача %q назначена на вас", task.Title),
	}

	// Уведомляем только предыдущего исполнителя
	if prevAssignee != nil {
		result[prevAssignee.ID] = fmt.Sprintf("Задача %q назначена на @%s",
			task.Title, user.Username)
	} else {
		// Уведомляем владельца только при первом назначении
		if task.Owner.ID != user.ID {
			result[task.Owner.ID] = fmt.Sprintf("Задача %q назначена на @%s",
				task.Title, user.Username)
		}
	}

	return result
}

// HandleUnassign снимает задачу с текущего исполнителя
func HandleUnassign(taskID int64, userID int64) map[int64]string {
	task, exists := r.GlobalMemory.GetByID(taskID)
	if !exists {
		return map[int64]string{userID: "Задача не найдена"}
	}

	if task.Assignee == nil {
		return map[int64]string{userID: "Задача не назначена"}
	}

	if task.Assignee.ID != userID {
		return map[int64]string{userID: "Задача не на вас"}
	}

	task.Assignee = nil
	r.GlobalMemory.Update(*task)

	return map[int64]string{
		userID:        "Принято",
		task.Owner.ID: fmt.Sprintf("Задача %q осталась без исполнителя", task.Title),
	}
}

// HandleResolve выполняет и удаляет задачу
func HandleResolve(taskID int64, userID int64) map[int64]string {
	task, exists := r.GlobalMemory.GetByID(taskID)
	if !exists {
		return map[int64]string{userID: "Задача не найдена"}
	}

	if task.Assignee == nil {
		return map[int64]string{userID: "Задача не назначена"}
	}

	if task.Assignee.ID != userID {
		return map[int64]string{userID: "Задача не на вас"}
	}

	r.GlobalMemory.Delete(taskID)

	return map[int64]string{
		userID:        fmt.Sprintf("Задача %q выполнена", task.Title),
		task.Owner.ID: fmt.Sprintf("Задача %q выполнена @%s", task.Title, task.Assignee.Username),
	}
}

// HandleMy возвращает задачи назначенные на пользователя
func HandleMy(userID int64) string {
	tasks := r.GlobalMemory.GetByAssignee(userID)
	if len(tasks) == 0 {
		return "Нет задач"
	}

	var result []string
	for _, task := range tasks {
		result = append(result, fmt.Sprintf("%d. %s by @%s\n/unassign_%d /resolve_%d",
			task.ID, task.Title, task.Owner.Username, task.ID, task.ID))
	}
	return strings.Join(result, "\n\n")
}

// HandleOwner возвращает задачи созданные пользователем
func HandleOwner(userID int64) string {
	tasks := r.GlobalMemory.GetByOwner(userID)
	if len(tasks) == 0 {
		return "Нет задач"
	}

	var result []string
	for _, task := range tasks {
		result = append(result, fmt.Sprintf("%d. %s by @%s\n/assign_%d",
			task.ID, task.Title, task.Owner.Username, task.ID))
	}
	return strings.Join(result, "\n\n")
}
