package telegram

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"taskbot/internal/router"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func StartTgBot(addr string, webhookURL string, token string) error {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return err
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(webhookURL))
	if err != nil {
		return err
	}

	log.Printf("Set webhook to %s", webhookURL)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Failed to read body: %v", err)
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var update tgbotapi.Update
		if err := json.Unmarshal(body, &update); err != nil {
			log.Printf("Failed to decode update: %v", err)
			http.Error(w, "Failed to decode update", http.StatusBadRequest)
			return
		}

		if update.Message == nil {
			log.Printf("Update contains no message")
			http.Error(w, "No message in update", http.StatusBadRequest)
			return
		}

		responses := router.Route(
			update.Message.Text,
			int64(update.Message.From.ID),
			update.Message.From.UserName,
		)

		for userID, response := range responses {
			msg := tgbotapi.NewMessage(userID, response)
			if _, err := bot.Send(msg); err != nil {
				log.Printf("Failed to send message to %d: %v", userID, err)
				continue
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: http.DefaultServeMux,
	}

	return srv.ListenAndServe()
}
