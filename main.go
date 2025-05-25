package main

import (
	tg "gopkg.in/telebot.v4"
	"log"
	"os"
	"time"
)

func main() {
	loadBotData()
	initProxy()

	settings := tg.Settings{
		Token:     os.Getenv("RWNOTIFY_TOKEN"),
		Poller:    &tg.LongPoller{Timeout: time.Second},
		ParseMode: "HTML",
	}

	var err error
	gBot, err = tg.NewBot(settings)
	if err != nil {
		log.Fatal("[f] failed to create Telegram bot: " + err.Error())
	}

	gBot.Handle("/start", sendHelp)
	gBot.Handle("/help", sendHelp)
	gBot.Handle("/codes", sendCodes)
	gBot.Handle("/add", processAddCommand)
	gBot.Handle("/list", processListCommand)
	gBot.Handle("/remove", processRemoveCommand)

	go updateRoutesLoop()
	gBot.Start()
}
