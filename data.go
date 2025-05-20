package main

import (
	tg "gopkg.in/telebot.v4"
	"sync"
)

type Route struct {
	number string
	from   string
	to     string
	date   string
}

type RouteInfo struct {
	valid          bool
	from           string
	to             string
	hasPlaces      bool
	hasLowerPlaces bool
}
type BotData struct {
	routeInfo  map[Route]RouteInfo
	routeUsers map[Route]map[int64]bool
	userRoutes map[int64]map[Route]bool
}

var gBot = tg.Bot{}
var gBotData = BotData{
	routeInfo:  map[Route]RouteInfo{},
	routeUsers: map[Route]map[int64]bool{},
	userRoutes: map[int64]map[Route]bool{},
}
var gBotMutex = sync.Mutex{}
