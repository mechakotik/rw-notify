package main

import (
	tg "gopkg.in/telebot.v4"
	"log"
)

func sendHelp(ctx tg.Context) error {
	help := "test"
	return ctx.Send(help)
}

func processAddCommand(ctx tg.Context) error {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	args := ctx.Args()
	if len(args) != 4 {
		return ctx.Send("Неправильный формат ввода, введите /help для справки")
	}

	var route Route
	route.number = args[0]
	route.from = args[1]
	route.to = args[2]
	route.date = args[3]

	info, exists := gBotData.routeInfo[route]
	if !exists {
		ctx.Send("Этот маршрут ещё не отслеживается ботом, получение данных с сервера...")
		info = fetchRouteInfo(route)
		if !info.valid {
			return ctx.Send("Сервер вернул невалидные данные, проверьте корректность ввода")
		}
		gBotData.routeInfo[route] = info
		log.Println("[l] added new route " + route.number + " (" + route.date + ") to global watchlist")
	}

	_, exists = gBotData.routeUsers[route]
	if !exists {
		gBotData.routeUsers[route] = map[int64]bool{}
	}
	gBotData.routeUsers[route][ctx.Sender().ID] = true

	_, exists = gBotData.userRoutes[ctx.Sender().ID]
	if !exists {
		gBotData.userRoutes[ctx.Sender().ID] = map[Route]bool{}
	}
	gBotData.userRoutes[ctx.Sender().ID][route] = true

	return ctx.Send("Теперь вы отслеживаете этот поезд")
}

func sendNotification(userID int64, route Route, old RouteInfo, new RouteInfo) {
	if gBot == nil {
		return
	}
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	user := &tg.User{
		ID: userID,
	}
	suffix := "в поезде " + route.number + "(" + route.date + ") "
	if !old.hasPlaces && new.hasPlaces {
		gBot.Send(user, "Появились свободные места "+suffix)
	}
	if old.hasPlaces && !new.hasPlaces {
		gBot.Send(user, "Больше нет свободных мест "+suffix)
	}
	if !old.hasLowerPlaces && new.hasLowerPlaces {
		gBot.Send(user, "Появились свободные нижние места "+suffix)
	}
	if old.hasLowerPlaces && !new.hasLowerPlaces {
		gBot.Send(user, "Больше нет свободных нижних мест "+suffix)
	}
}
