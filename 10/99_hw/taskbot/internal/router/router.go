package router

import (
	"strconv"
	"strings"
	"taskbot/internal/models"
	"taskbot/internal/repository"
	"taskbot/internal/service"
)

// В пакете хранится маршрутизация команд.
// Аналог автоматической маршрутизации в веб приложениях.

// Route обрабатывает входящее сообщение и возвращает текст ответа
// text - текст команды, полученной от пользователя
// userID - идентификатор пользователя, отправившего команду
// username - имя пользователя, отправившего команду
// Возвращает карту с ID пользователей и их ответами
func Route(text string, userID int64, username string) map[int64]string {
	// Сохраняем пользователя при каждом запросе
	repository.GlobalMemory.AddUser(&models.User{
		ID:       userID,
		Username: username,
	})

	// Парсим команду
	switch {
	case text == "/tasks":
		// Возвращаем список всех задач
		return map[int64]string{userID: service.HandleTasks(userID)}

	case text == "/my":
		// Возвращаем задачи, назначенные на пользователя
		return map[int64]string{userID: service.HandleMy(userID)}

	case text == "/owner":
		// Возвращаем задачи, созданные пользователем
		return map[int64]string{userID: service.HandleOwner(userID)}

	case strings.HasPrefix(text, "/new "):
		// Создаем новую задачу
		title := strings.TrimPrefix(text, "/new ")
		return map[int64]string{userID: service.HandleNew(title, &models.User{ID: userID, Username: username})}

	case strings.HasPrefix(text, "/assign_"):
		// Назначаем задачу на пользователя
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/assign_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleAssign(taskID, &models.User{ID: userID, Username: username})

	case strings.HasPrefix(text, "/unassign_"):
		// Снимаем задачу с пользователя
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/unassign_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleUnassign(taskID, userID)

	case strings.HasPrefix(text, "/resolve_"):
		// Выполняем и удаляем задачу
		taskID, err := strconv.ParseInt(strings.TrimPrefix(text, "/resolve_"), 10, 64)
		if err != nil {
			return map[int64]string{userID: "Некорректный ID задачи"}
		}
		return service.HandleResolve(taskID, userID)

	default:
		// Неизвестная команда
		return map[int64]string{userID: "Неизвестная команда"}
	}
}
