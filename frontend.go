package main

import (
	"fmt"
	tg "gopkg.in/telebot.v4"
	"log"
	"strconv"
	"time"
	"unicode/utf8"
)

func sendHelp(ctx tg.Context) error {
	help := "<b>/add [number] [from] [to] [date]</b>\n" +
		"Добавить маршрут в отслеживание\n" +
		"<b>[number]</b> - номер поезда, например 704Б\n" +
		"<b>[from]</b> - код станции отправления, см. /codes\n" +
		"<b>[to]</b> - код станции прибытия, см. /codes\n" +
		"<b>[date]</b> - дата отправления, например 2025-05-25\n\n" +
		"<b>/list</b>\n" +
		"Список всех маршрутов, которые вы отслеживаете\n\n" +
		"<b>/remove [index]</b>\n" +
		"Убрать маршрут из отслеживания\n" +
		"<b>[index]</b> - номер маршрута в выводе команды /list\n"

	return ctx.Send(help)
}

func sendCodes(ctx tg.Context) error {
	codes := "БЖД использует коды железнодорожных станций в формате АСУ \"Экспресс-3\" (UIC-коды)." +
		"Вот такие коды для станций в областных центрах:\n\n" +
		"<b>2100001</b> - Минск-Пассажирский\n" +
		"<b>2100200</b> - Брест-Центральный\n" +
		"<b>2100050</b> - Витебск\n" +
		"<b>2100100</b> - Гомель\n" +
		"<b>2100070</b> - Гродно\n" +
		"<b>2100150</b> - Могилёв\n\n" +
		"Чтобы посмотреть код другой станции, можно открыть на pass.rw.by " +
		"список поездов на каком-то маршруте, прибывающем на эту станцию, " +
		"и посмотреть на число после &to_exp= в ссылке на страницу."

	return ctx.Send(codes)
}

func processAddCommand(ctx tg.Context) error {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	args := ctx.Args()
	if len(args) != 4 {
		return ctx.Send("Неправильное количество аргументов, введите /help для справки")
	}

	var route Route
	route.Number = args[0]
	route.From = args[1]
	route.To = args[2]
	route.Date = args[3]

	if !isValidTrainNumber(route.Number) {
		return ctx.Send("Некорректный формат номера поезда, введите /help для справки")
	}
	if !isValidStationCode(route.From) {
		return ctx.Send("Некорректный код станции отправления, введите /help для справки")
	}
	if !isValidStationCode(route.To) {
		return ctx.Send("Некорректный код станции прибытия, введите /help для справки")
	}
	if !isValidDate(route.Date) {
		return ctx.Send("Некорректная дата, введите /help для справки")
	}

	info, exists := gBotData.RouteInfo[route]
	if !exists {
		ctx.Send("Этот маршрут ещё не отслеживается ботом, получение данных с сервера...")
		info = fetchRouteInfo(route)
		if !info.Valid {
			return ctx.Send("Сервер вернул невалидные данные, проверьте корректность ввода")
		}
		gBotData.RouteInfo[route] = info
		log.Println("[l] added new route " + route.Number + " (" + route.Date + ") To global watchlist")
	}

	_, exists = gBotData.RouteUsers[route]
	if !exists {
		gBotData.RouteUsers[route] = map[int64]bool{}
	}
	gBotData.RouteUsers[route][ctx.Sender().ID] = true

	_, exists = gBotData.UserRoutes[ctx.Sender().ID]
	if !exists {
		gBotData.UserRoutes[ctx.Sender().ID] = map[Route]bool{}
	}
	gBotData.UserRoutes[ctx.Sender().ID][route] = true

	defer saveBotData()
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
	layout := "2006-01-02"
	parsedTime, err := time.Parse(layout, date)
	if err != nil {
		return false
	}

	currentTime := time.Now().UTC()
	diff := currentTime.Sub(parsedTime)
	if diff < -48 || diff > 768 {
		return false
	}

	return true
}

func processListCommand(ctx tg.Context) error {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	routes, exists := gBotData.UserRoutes[ctx.Sender().ID]
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
			message += fmt.Sprintf("%d. %s %s %s %s\n", index, route.Number, route.From, route.To, route.Date)
		}
	}

	return ctx.Send(message)
}

func processRemoveCommand(ctx tg.Context) error {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	args := ctx.Args()
	if len(args) != 1 {
		return ctx.Send("Неправильный формат ввода, введите /help для справки")
	}
	remIndex, err := strconv.Atoi(args[0])
	if err != nil {
		return ctx.Send("Неправильный формат ввода, введите /help для справки")
	}

	routes, exists := gBotData.UserRoutes[ctx.Sender().ID]
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

	gBotData.RouteUsers[remRoute][ctx.Sender().ID] = false
	gBotData.UserRoutes[ctx.Sender().ID][remRoute] = false

	defer saveBotData()
	return ctx.Send(fmt.Sprintf("Маршрут %s (%s) больше не отслеживается", remRoute.Number, remRoute.Date))
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
	suffix := "в поезде " + route.Number + " (" + route.Date + ") "
	if !old.HasPlaces && new.HasPlaces {
		gBot.Send(user, "Появились свободные места "+suffix)
	}
	if old.HasPlaces && !new.HasPlaces {
		gBot.Send(user, "Больше нет свободных мест "+suffix)
	}
	if !old.HasLowerPlaces && new.HasLowerPlaces {
		gBot.Send(user, "Появились свободные нижние места "+suffix)
	}
	if old.HasLowerPlaces && !new.HasLowerPlaces {
		gBot.Send(user, "Больше нет свободных нижних мест "+suffix)
	}
}
