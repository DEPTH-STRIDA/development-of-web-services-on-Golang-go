package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync/atomic"

	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	// импорт с переименованием.
	// tgbotapi используется в примерах из документации.
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func init() {
	// upd global var for testing
	// we use patched version of gopkg.in/telegram-bot-api.v4 ( WebhookURL const -> var)
	WebhookURL = "http://127.0.0.1:8081"
	BotToken = "_golangcourse_test"
}

// HTTP клиент с таймаутом в 1 секунду
// Используется для отправки запросов к тестовому серверу
// Timeout определяет максимальное время ожидания для всего HTTP запроса (включая чтение ответа)
var client = &http.Client{Timeout: time.Second}

// TDS (Telegram Dummy Server) - мок-сервер, имитирующий Telegram Bot API
// Mutex используется для безопасного доступа к map из разных горутин
// Answers хранит ответы бота для разных пользователей (ключ - ID пользователя)
type TDS struct {
	*sync.Mutex
	Answers map[int]string
}

// NewTDS создает новый экземпляр мок-сервера
func NewTDS() *TDS {
	return &TDS{
		Mutex:   &sync.Mutex{},
		Answers: make(map[int]string),
	}
}

// ServeHTTP обрабатывает HTTP запросы к мок-серверу
// Реализует основные методы Telegram Bot API:
// - /getMe - информация о боте
// - /setWebhook - установка вебхука
// - /sendMessage - отправка сообщения
func (srv *TDS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// mux - это маршрутизатор, который используется для обработки HTTP запросов.
	// Он создается с помощью http.NewServeMux() и используется для маршрутизации запросов к различным обработчикам.
	mux := http.NewServeMux()

	// Обработчик для метода /getMe
	// Возвращает информацию о боте в формате JSON
	mux.HandleFunc("/getMe", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":{"id":` +
			strconv.Itoa(BotChatID) +
			`,"is_bot":true,"first_name":"game_test_bot","username":"game_test_bot"}}`))
	})

	// Обработчик для метода /setWebhook
	// Возвращает true, если вебхук был успешно установлен
	mux.HandleFunc("/setWebhook", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true,"result":true,"description":"Webhook was set"}`))
	})

	mux.HandleFunc("/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		chatID, _ := strconv.Atoi(r.FormValue("chat_id"))
		text := r.FormValue("text")
		srv.Lock()
		srv.Answers[chatID] = text
		srv.Unlock()

		//fmt.Println("TDS sendMessage", chatID, text)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		panic(fmt.Errorf("unknown command %s", r.URL.Path))
	})

	handler := http.StripPrefix("/bot"+BotToken, mux)
	handler.ServeHTTP(w, r)
}

// Константы для тестовых пользователей
const (
	// ID пользователей
	Ivanov     int = 256
	Petrov     int = 512
	Alexandrov int = 1024
	// ID бота
	BotChatID = 100500
)

var (
	// Пользователи
	users = map[int]*tgbotapi.User{
		Ivanov: {
			ID:           Ivanov,
			FirstName:    "Ivan",
			LastName:     "Ivanov",
			UserName:     "ivanov",
			LanguageCode: "ru",
			IsBot:        false,
		},
		Petrov: {
			ID:           Petrov,
			FirstName:    "Petr",
			LastName:     "Pertov",
			UserName:     "ppetrov",
			LanguageCode: "ru",
			IsBot:        false,
		},
		Alexandrov: {
			ID:           Alexandrov,
			FirstName:    "Alex",
			LastName:     "Alexandrov",
			UserName:     "aalexandrov",
			LanguageCode: "ru",
			IsBot:        false,
		},
	}

	// ID обновления и сообщения
	updID uint64
	msgID uint64
)

// SendMsgToBot эмулирует отправку сообщения боту от пользователя
// userID - ID отправителя
// text - текст сообщения
// Создает объект Update, как это делает настоящий Telegram, и отправляет его боту
func SendMsgToBot(userID int, text string) error {
	// Увеличиваем ID обновления и сообщения
	atomic.AddUint64(&updID, 1)
	myUpdID := atomic.LoadUint64(&updID)

	// Увеличиваем ID сообщения
	atomic.AddUint64(&msgID, 1)
	myMsgID := atomic.LoadUint64(&msgID)

	// Получаем пользователя из карты
	user, ok := users[userID]
	if !ok {
		return fmt.Errorf("no user %v", userID)
	}

	// Создаем объект обновления
	upd := &tgbotapi.Update{
		UpdateID: int(myUpdID),
		Message: &tgbotapi.Message{
			MessageID: int(myMsgID),
			From:      user,
			Chat: &tgbotapi.Chat{
				ID:        int64(user.ID),
				FirstName: user.FirstName,
				UserName:  user.UserName,
				Type:      "private",
			},
			Text: text,
			Date: int(time.Now().Unix()),
		},
	}
	reqData, _ := json.Marshal(upd)

	// Отправляем запрос на вебхук
	reqBody := bytes.NewBuffer(reqData)
	req, _ := http.NewRequest(http.MethodPost, WebhookURL, reqBody)
	_, err := client.Do(req)
	return err
}

// Структура для хранения тестовых случаев
type testCase struct {
	user    int
	command string
	answers map[int]string
}

func TestTasks(t *testing.T) {
	// Создаем мок-сервер
	tds := NewTDS()
	ts := httptest.NewServer(tds)
	tgbotapi.APIEndpoint = ts.URL + "/bot%s/%s"

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Канал для передачи ошибок
	errCh := make(chan error, 1)

	// Запускаем бота в отдельной горутине
	go func() {
		if err := startTaskBot(ctx, ":8081"); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()

	// Ждем запуск бота
	time.Sleep(500 * time.Millisecond)

	// Проверяем, что бот запустился без ошибок
	select {
	case err := <-errCh:
		t.Fatalf("startTaskBot error: %s", err)
	default:
	}

	// Тестовые случаи
	cases := []testCase{
		// Иванов смотрим задачи, создает задачу, смотрит задачи
		{
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: "Нет задач",
			},
		},
		{
			Ivanov,
			"/new написать бота",
			map[int]string{
				Ivanov: `Задача "написать бота" создана, id=1`,
			},
		},
		{
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: `1. написать бота by @ivanov
/assign_1`,
			},
		},
		// Александров назначает себя на задачу. В ответ получает сообщение о назначении
		// + сообщение Иванову о назначении (владельцу, если не было исполнителя)
		{
			Alexandrov,
			"/assign_1",
			map[int]string{
				Alexandrov: `Задача "написать бота" назначена на вас`,
				Ivanov:     `Задача "написать бота" назначена на @aalexandrov`,
			},
		},
		// Петров назначает себя на задачу. В ответ получает сообщение о назначении
		// + сообщение Александрову о назначении (предыдущему исполнителю)
		{
			Petrov,
			"/assign_1",
			map[int]string{
				Petrov:     `Задача "написать бота" назначена на вас`,
				Alexandrov: `Задача "написать бота" назначена на @ppetrov`,
			},
		},

		{
			Alexandrov,
			"/tasks",
			map[int]string{
				Petrov: `1. написать бота by @ivanov
assignee: я
/unassign_1 /resolve_1`,
			},
		},
		{
			Ivanov,
			"/tasks",
			map[int]string{
				Ivanov: `1. написать бота by @ivanov
assignee: @ppetrov`,
			},
		},
		// Александров снимает задачу с себя, но не может потому что она не на нём
		{
			Alexandrov,
			"/unassign_1",
			map[int]string{
				Alexandrov: `Задача не на вас`,
			},
		},
		// Петров снимает задачу с себя, она остаётся без исполнителя
		{
			Petrov,
			"/unassign_1",
			map[int]string{
				Petrov: `Принято`,
				Ivanov: `Задача "написать бота" осталась без исполнителя`,
			},
		},
		{
			// Как и в предыдущем случае, Петров назначает задачу на себя И ЕСЛИ НЕТ ИСПОЛНИТЕЛЯ, то уведомление владельцу
			Petrov,
			"/assign_1",
			map[int]string{
				Petrov: `Задача "написать бота" назначена на вас`,
				Ivanov: `Задача "написать бота" назначена на @ppetrov`,
			},
		},
		{
			Petrov,
			"/resolve_1",
			map[int]string{
				Petrov: `Задача "написать бота" выполнена`,
				Ivanov: `Задача "написать бота" выполнена @ppetrov`,
			},
		},
		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `Нет задач`,
			},
		},
		{
			Petrov,
			"/new сделать ДЗ по курсу",
			map[int]string{
				Petrov: `Задача "сделать ДЗ по курсу" создана, id=2`,
			},
		},
		{
			Ivanov,
			"/new прийти на хакатон",
			map[int]string{
				Ivanov: `Задача "прийти на хакатон" создана, id=3`,
			},
		},
		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
/assign_2

3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
		{
			Petrov,
			"/assign_2",
			map[int]string{
				Petrov: `Задача "сделать ДЗ по курсу" назначена на вас`,
			},
		},
		{
			Petrov,
			"/tasks",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
assignee: я
/unassign_2 /resolve_2

3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
		{
			Petrov,
			"/my",
			map[int]string{
				Petrov: `2. сделать ДЗ по курсу by @ppetrov
/unassign_2 /resolve_2`,
			},
		},
		{
			Ivanov,
			"/owner",
			map[int]string{
				Ivanov: `3. прийти на хакатон by @ivanov
/assign_3`,
			},
		},
	}

	// Выполняем тесты
	for idx, item := range cases {
		// Очищаем ответы перед каждым тестом
		tds.Lock()
		tds.Answers = make(map[int]string)
		tds.Unlock()

		// Формируем имя теста
		caseName := fmt.Sprintf("[case%d, %d: %s]", idx, item.user, item.command)

		// Отправляем сообщение боту
		err := SendMsgToBot(item.user, item.command)
		if err != nil {
			t.Fatalf("%s SendMsgToBot error: %s", caseName, err)
		}

		// Даем время на обработку запроса
		time.Sleep(10 * time.Millisecond)

		// Проверяем ответы
		tds.Lock()
		result := reflect.DeepEqual(tds.Answers, item.answers)
		if !result {
			t.Fatalf("%s bad results:\n\tWant: %v\n\tHave: %v", caseName, item.answers, tds.Answers)
		}
		tds.Unlock()
	}
}
