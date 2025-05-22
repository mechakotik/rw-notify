package main

import (
	"fmt"
	tg "gopkg.in/telebot.v4"
	"log"
	"strconv"
	"unicode/utf8"
)

func sendHelp(ctx tg.Context) error {
	help := "/add [number] [from] [to] [date]\n" +
		"Добавить маршрут в отслеживание\n" +
		"[number] - номер поезда, например 704Б\n" +
		"[from] - код станции отправления в ЕСР, например 2100050\n" +
		"[to] - код станции прибытия в ЕСР, например 2100001\n" +
		"[date] - дата отправления, например 2025-05-25\n\n" +
		"/list\n" +
		"Список всех маршрутов, которые вы отслеживаете\n\n" +
		"/remove [index]\n" +
		"Убрать маршрут из отслеживания\n" +
		"[index] - номер маршрута в выводе команды /list\n"

	return ctx.Send(help)
}

func processAddCommand(ctx tg.Context) error {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	args := ctx.Args()
	if len(args) != 4 {
		return ctx.Send("Неправильное количество аргументов, введите /help для справки")
	}

	var route Route
	route.number = args[0]
	route.from = args[1]
	route.to = args[2]
	route.date = args[3]

	if !isValidTrainNumber(route.number) {
		return ctx.Send("Некорректный формат номера поезда, введите /help для справки")
	}
	if !isValidStationCode(route.from) {
		return ctx.Send("Некорректный код станции отправления, введите /help для справки")
	}
	if !isValidStationCode(route.to) {
		return ctx.Send("Некорректный код станции прибытия, введите /help для справки")
	}
	if !isValidDate(route.date) {
		return ctx.Send("Некорректная дата, введите /help для справки")
	}

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

	return ctx.Send("Теперь вы отслеживаете этот маршрут")
}

func isValidTrainNumber(number string) bool {
	if utf8.RuneCountInString(number) != 4 {
		return false
	}
	for i := 0; i < 3; i++ {
		if !('0' <= number[i] && number[i] <= '9') {
			return false
		}
	}
	return true
}

func isValidStationCode(code string) bool {
	for i := 0; i < len(code); i++ {
		if !('0' <= code[i] && code[i] <= '9') {
			return false
		}
	}
	return true
}

func isValidDate(date string) bool {
	if len(date) != 10 {
		return false
	}
	for _, i := range [](int){0, 1, 2, 3, 5, 6, 8, 9} {
		if !('0' <= date[i] && date[i] <= '9') {
			return false
		}
	}
	for _, i := range [](int){4, 7} {
		if date[i] != '-' {
			return false
		}
	}
	return true
}

func processListCommand(ctx tg.Context) error {
	routes, exists := gBotData.userRoutes[ctx.Sender().ID]

	hasRoutes := false
	if exists {
		for _, enabled := range routes {
			if enabled {
				hasRoutes = true
				break
			}
		}
	}
	if !hasRoutes {
		return ctx.Send("Вы не отслеживаете никакие маршруты")
	}

	message := ""
	index := 0
	for route, enabled := range routes {
		if enabled {
			index++
			message += fmt.Sprintf("%d. %s %s-%s %s\n", index, route.number, route.from, route.to, route.date)
		}
	}

	return ctx.Send(message)
}

func processRemoveCommand(ctx tg.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return ctx.Send("Неправильный формат ввода, введите /help для справки")
	}
	remIndex, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send("Неправильный формат ввода, введите /help для справки")
	}

	routes, exists := gBotData.userRoutes[ctx.Sender().ID]
	if !exists {
		return ctx.Send("Вы не отслеживаете никакие маршруты")
	}

	index := 0
	remRoute := Route{}
	for route, enabled := range routes {
		if enabled {
			index++
			if index == remIndex {
				remRoute = route
				index = -1
				break
			}
		}
	}
	if index != -1 {
		return ctx.Send("Вы не отслеживаете маршрут с номером " + strconv.Itoa(remIndex) + ", введите /list чтобы узнать нужный номер")
	}

	gBotData.routeUsers[remRoute][ctx.Sender().ID] = false
	gBotData.userRoutes[ctx.Sender().ID][remRoute] = false
	return ctx.Send(fmt.Sprintf("Маршрут %s (%s) больше не отслеживается", remRoute.number, remRoute.date))
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
	suffix := "в поезде " + route.number + " (" + route.date + ") "
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
