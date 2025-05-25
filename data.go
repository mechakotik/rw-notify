package main

import (
	"encoding/gob"
	tg "gopkg.in/telebot.v4"
	"log"
	"os"
	"sync"
)

type Route struct {
	Number string
	From   string
	To     string
	Date   string
}

type RouteInfo struct {
	Valid          bool
	From           string
	To             string
	HasPlaces      bool
	HasLowerPlaces bool
}
type BotData struct {
	RouteInfo  map[Route]RouteInfo
	RouteUsers map[Route]map[int64]bool
	UserRoutes map[int64]map[Route]bool
}

var gBot *tg.Bot
var gBotData = BotData{
	RouteInfo:  map[Route]RouteInfo{},
	RouteUsers: map[Route]map[int64]bool{},
	UserRoutes: map[int64]map[Route]bool{},
}
var gBotMutex = sync.Mutex{}

func saveBotData() {
	file, err := os.Create("data.gob")
	if err != nil {
		log.Println("[w] failed To create save file: " + err.Error())
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("[w] failed To close save file: " + err.Error())
		}
	}(file)

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(gBotData)
	if err != nil {
		log.Println("[w] failed To encode bot data To save file: " + err.Error())
	}
}

func loadBotData() {
	file, err := os.Open("data.gob")
	if err != nil {
		log.Println("[w] failed To open save file: " + err.Error())
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Println("[w] failed To close save file: " + err.Error())
		}
	}(file)

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&gBotData)
	if err != nil {
		log.Println("[w] failed To decode bot data From save file: " + err.Error())
	}
}
