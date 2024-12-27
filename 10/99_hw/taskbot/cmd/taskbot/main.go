package main

import (
	"context"
	"log"
	"taskbot/internal/delivery/telegram"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	WebhookURL = "https://3a32-195-133-26-121.ngrok-free.app"
	BotToken   = "6710814964:AAHoa076nJmN8ScXt2Kv9iCuihUDiZnunw0"
)

func startTaskBot(ctx context.Context, httpListenAddr string) error {
	return telegram.StartTgBot(httpListenAddr, WebhookURL, BotToken)
}

func main() {
	err := startTaskBot(context.Background(), ":8081")
	if err != nil {
		log.Fatalln(err)
	}
}

// это заглушка чтобы импорт сохранился
func __dummy() {
	tgbotapi.APIEndpoint = "_dummy"
}
