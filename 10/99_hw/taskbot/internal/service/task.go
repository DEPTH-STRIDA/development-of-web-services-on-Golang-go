package service

import (
	"fmt"
	"sort"
	"strings"
	"taskbot/internal/models"
	r "taskbot/internal/repository"
)

// В пакете хранится бизнес логика. По обработчику на каждую команду.
// Для сложения используется strings.Builder. Самый быстрый способ + понятный.

// HandleTasks возвращает список всех задач для указанного пользователя.
// userID - идентификатор пользователя, для которого нужно получить задачи.
// Возвращает строку с описанием задач или сообщение о том, что задач нет.
func HandleTasks(userID int64) string {
	tasks := r.GlobalMemory.GetAll()
	if len(tasks) == 0 {
		return "Нет задач"
	}

	// Сортировка задач по ID
	sort.Sort(models.TaskSlice(tasks))

	var builder strings.Builder
	for _, task := range tasks {
		if task.Assignee != nil && task.Assignee.ID == userID {
			builder.WriteString(fmt.Sprintf("%d. %s by @%s\nassignee: я\n/unassign_%d /resolve_%d\n\n",
				task.ID, task.Title, task.Owner.Username, task.ID, task.ID))
		} else if task.Assignee != nil {
			builder.WriteString(fmt.Sprintf("%d. %s by @%s\nassignee: @%s",
				task.ID, task.Title, task.Owner.Username, task.Assignee.Username))
		} else {
			builder.WriteString(fmt.Sprintf("%d. %s by @%s\n/assign_%d",
				task.ID, task.Title, task.Owner.Username, task.ID))
		}
	}
	return builder.String()
}

// HandleNew создает новую задачу с указанным заголовком и владельцем.
// title - заголовок задачи.
// owner - пользователь, который является владельцем задачи.
// Возвращает строку с подтверждением создания задачи и её ID.
func HandleNew(title string, owner *models.User) string {
	task := models.Task{
		Title: title,
		Owner: owner,
	}
	id := r.GlobalMemory.Create(task)
	return fmt.Sprintf("Задача %q создана, id=%d", title, id)
}

// HandleAssign назначает задачу с указанным ID на пользователя.
// taskID - идентификатор задачи.
// user - пользователь, на которого назначается задача.
// Возвращает карту с ID пользователей и сообщениями для них.
func HandleAssign(taskID int64, user *models.User) map[int64]string {
	task, exists := r.GlobalMemory.GetByID(taskID)
	if !exists {
		return map[int64]string{user.ID: "Задача не найдена"}
	}

	prevAssignee := task.Assignee
	task.Assignee = user
	r.GlobalMemory.Update(task)

	result := map[int64]string{
		user.ID: fmt.Sprintf("Задача %q назначена на вас", task.Title),
	}

	if prevAssignee != nil {
		result[prevAssignee.ID] = fmt.Sprintf("Задача %q назначена на @%s",
			task.Title, user.Username)
	} else {
		if task.Owner.ID != user.ID {
			result[task.Owner.ID] = fmt.Sprintf("Задача %q назначена на @%s",
				task.Title, user.Username)
		}
	}

	return result
}

// HandleUnassign снимает задачу с текущего исполнителя.
// taskID - идентификатор задачи.
// userID - идентификатор пользователя, который снимает задачу.
// Возвращает карту с ID пользователей и сообщениями для них.
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
	r.GlobalMemory.Update(task)

	return map[int64]string{
		userID:        "Принято",
		task.Owner.ID: fmt.Sprintf("Задача %q осталась без исполнителя", task.Title),
	}
}

// HandleResolve выполняет и удаляет задачу с указанным ID.
// taskID - идентификатор задачи.
// userID - идентификатор пользователя, который выполняет задачу.
// Возвращает карту с ID пользователей и сообщениями для них.
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

// HandleMy возвращает задачи, назначенные на указанного пользователя.
// userID - идентификатор пользователя.
// Возвращает строку с описанием задач или сообщение о том, что задач нет.
func HandleMy(userID int64) string {
	tasks := r.GlobalMemory.GetByAssignee(userID)
	if len(tasks) == 0 {
		return "Нет задач"
	}

	// Сортировка задач по ID
	sort.Sort(models.TaskSlice(tasks))

	var builder strings.Builder
	for _, task := range tasks {
		builder.WriteString(fmt.Sprintf("%d. %s by @%s\n/unassign_%d /resolve_%d\n\n",
			task.ID, task.Title, task.Owner.Username, task.ID, task.ID))
	}
	return builder.String()
}

// HandleOwner возвращает задачи, созданные указанным пользователем.
// userID - идентификатор пользователя.
// Возвращает строку с описанием задач или сообщение о том, что задач нет.
func HandleOwner(userID int64) string {
	tasks := r.GlobalMemory.GetByOwner(userID)
	if len(tasks) == 0 {
		return "Нет задач"
	}

	// Сортировка задач по ID
	sort.Sort(models.TaskSlice(tasks))

	var builder strings.Builder
	for _, task := range tasks {
		builder.WriteString(fmt.Sprintf("%d. %s by @%s\n/assign_%d\n\n",
			task.ID, task.Title, task.Owner.Username, task.ID))
	}
	return builder.String()
}
