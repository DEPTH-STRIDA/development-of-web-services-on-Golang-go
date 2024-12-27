package router

import (
	"strconv"
	"strings"
	"taskbot/internal/models"
	"taskbot/internal/repository"
	"taskbot/internal/service"
)

// Route обрабатывает входящее сообщение и возвращает текст ответа
func Route(text string, userID int64, username string) map[int64]string {
	// Сохраняем пользователя при каждом запросе
	repository.GlobalMemory.AddUser(&models.User{
		ID:       userID,
		Username: username,
	})

	// Парсим команду
	switch {
	case text == "/tasks":
		return map[int64]string{userID: service.HandleTasks(userID)}

	case text == "/my":
		return map[int64]string{userID: service.HandleMy(userID)}

	case text == "/owner":
		return map[int64]string{userID: service.HandleOwner(userID)}

	case strings.HasPrefix(text, "/new "):
		title := strings.TrimPrefix(text, "/new ")
		return map[int64]string{userID: service.HandleNew(title, &models.User{ID: userID, Username: username})}

	case strings.HasPrefix(text, "/assign_"):
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/assign_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleAssign(taskID, &models.User{ID: userID, Username: username})

	case strings.HasPrefix(text, "/unassign_"):
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/unassign_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleUnassign(taskID, userID)

	case strings.HasPrefix(text, "/resolve_"):
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/resolve_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleResolve(taskID, userID)

	default:
		return map[int64]string{userID: "Неизвестная команда"}
	}
}
