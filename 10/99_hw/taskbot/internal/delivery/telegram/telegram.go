package telegram

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"taskbot/internal/router"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartTgBot запускает Telegram бота с использованием вебхуков
// addr - адрес, на котором будет запущен HTTP сервер
// webhookURL - URL для установки вебхука
// token - токен бота, полученный от BotFather
func StartTgBot(addr string, webhookURL string, token string) error {
	bot, err := initializeBot(token, webhookURL)
	if err != nil {
		return err
	}

	// Добавление обработчика вебхука в стандартный маршрутизатор.

	// На самом деле это не очень хорошая практика, но для простоты мы так и сделали.
	// Обработчик стоит выделить в отдельную переменную, как и обработчики. (см. example/server.go)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(bot, w, r)
	})

	return startHTTPServer(addr)
}

// initializeBot инициализирует бота и устанавливает вебхук
func initializeBot(token string, webhookURL string) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Установка вебхука, чтобы бот мог получать сообщения от телеграма через вебхук
	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		return nil, err
	}

	log.Printf("Set webhook to %s", webhookURL)
	return bot, nil
}

// handleWebhook обрабатывает входящие вебхуки
func handleWebhook(bot *tgbotapi.BotAPI, w http.ResponseWriter, r *http.Request) {
	// Проверка, что метод запроса POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Чтение тела запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Декодирование тела запроса в структуру Update
	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		log.Printf("Failed to decode update: %v", err)
		http.Error(w, "Failed to decode update", http.StatusBadRequest)
		return
	}

	// Проверка, что в обновлении есть сообщение
	if update.Message == nil {
		log.Printf("Update contains no message")
		http.Error(w, "No message in update", http.StatusBadRequest)
		return
	}

	// Обработка обновления
	// Запуск процесса обработки в отдельной горутине, чтобы не блокировать HTTP сервер
	// Бизнес логика может занять больше времени. Такое стоит учитывать при разработке сайтов, ведь
	// пока функция не завершится, HTTP сервер не сможет обработать следующий запрос. Для пользователя это будет
	// казаться, что сервер грузится.
	go processUpdate(bot, update)

	// Отправка ответа в телеграм
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// processUpdate обрабатывает обновление и отправляет ответы
func processUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Метод Route из пакета router.go
	// Принимает текст сообщения, id пользователя, имя пользователя
	// Возвращает мапу с id пользователей и ответами
	responses := router.Route(
		update.Message.Text,
		int64(update.Message.From.ID),
		update.Message.From.UserName,
	)

	// Отправка ответов пользователям
	for userID, response := range responses {

		// msg - структура config для отправки сообщения (реализует интерфейс Config)
		msg := tgbotapi.NewMessage(userID, response)

		// Отправка сообщения. Send принимает интерфейс Config, а не конкретную структуру
		if _, err := bot.Send(msg); err != nil {
			log.Printf("Failed to send message to %d: %v", userID, err)
			continue
		}
	}
}

// startHTTPServer запускает HTTP сервер
func startHTTPServer(addr string) error {
	srv := &http.Server{
		Addr: addr,
		// Стандартный обработчик маршрутов, туда мы и добавили обработчик.
		Handler: http.DefaultServeMux,
	}

	return srv.ListenAndServe()
}
