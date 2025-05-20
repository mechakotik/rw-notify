package main

import (
	tg "gopkg.in/telebot.v4"
	"log"
	"os"
	"time"
)

func main() {
	settings := tg.Settings{
		Token:  os.Getenv("TOKEN"),
		Poller: &tg.LongPoller{Timeout: time.Second},
	}

	gBot, err := tg.NewBot(settings)
	if err != nil {
		log.Fatal("[f] failed to create Telegram bot: " + err.Error())
	}

	gBot.Handle("/start", sendHelp)
	gBot.Handle("/help", sendHelp)
	gBot.Handle("/add", processAddCommand)

	go updateRoutesLoop()
	gBot.Start()
}
