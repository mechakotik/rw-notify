package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/net/context"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

var client *http.Client

func initProxy() {
	proxyIP := os.Getenv("RWNOTIFY_PROXY_IP")
	if proxyIP == "" {
		return
	}

	var auth *proxy.Auth
	proxyUser := os.Getenv("RWNOTIFY_PROXY_USER")
	proxyPassword := os.Getenv("RWNOTIFY_PROXY_PASSWORD")
	if proxyUser != "" || proxyPassword != "" {
		auth = &proxy.Auth{
			User:     os.Getenv("RWNOTIFY_PROXY_USER"),
			Password: os.Getenv("RWNOTIFY_PROXY_PASSWORD"),
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyIP, auth, &net.Dialer{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatal("[f] failed to create dialer: " + err.Error())
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}
	client = &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	log.Println("[l] using proxy " + proxyIP)
}

func formRouteURL(route Route, carType int) string {
	urlValues := url.Values{}
	urlValues.Add("from", route.From)
	urlValues.Add("to", route.To)
	urlValues.Add("date", route.Date)
	urlValues.Add("train_number", route.Number)
	urlValues.Add("car_type", strconv.Itoa(carType))

	routeBaseURL := "https://pass.rw.by/ru/ajax/route/car_places"
	routeURL := routeBaseURL + "?" + urlValues.Encode()
	return routeURL
}

func fetchJSON(url string) map[string]interface{} {
	var resp *http.Response
	var err error
	if client != nil {
		resp, err = client.Get(url)
	} else {
		resp, err = http.Get(url)
	}

	if err != nil {
		log.Println("[e] error when fetching JSON from " + url + ": " + err.Error())
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		log.Println("[e] error when fetching JSON from " + url + ": HTTP status code " + strconv.Itoa(resp.StatusCode))
		return nil
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("[w] failed to close response body: " + err.Error())
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[e] error when reading fetched JSON: " + err.Error())
		return nil
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Println("[e] error when parsing fetched JSON: " + err.Error())
		return nil
	}

	return result
}

func fetchRouteInfo(route Route) RouteInfo {
	result := RouteInfo{
		Valid:          false,
		HasPlaces:      false,
		HasLowerPlaces: false,
	}

	// TODO: all Valid car types here
	for _, carType := range [](int){3, 4} {
		mp := fetchJSON(formRouteURL(route, carType))
		if mp != nil {
			result.Valid = true
		}
		tariffs, ok := mp["tariffs"].([]interface{})
		if !ok {
			continue
		}

		for _, tariff := range tariffs {
			cars, ok := tariff.(map[string]interface{})["cars"].([]interface{})
			if !ok {
				continue
			}
			for _, car := range cars {
				totalPlaces, ok := car.(map[string]interface{})["totalPlaces"].(float64)
				if ok && totalPlaces != 0 {
					result.HasPlaces = true
				}
				lowerPlaces, ok := car.(map[string]interface{})["lowerPlaces"].(float64)
				if ok && lowerPlaces != 0 {
					result.HasLowerPlaces = true
				}
			}
		}
	}

	return result
}

func updateRoutesLoop() {
	for {
		updateRoutesInfo()
		time.Sleep(time.Minute)
	}
}

func updateRoutesInfo() {
	gBotMutex.Lock()
	defer gBotMutex.Unlock()

	dropRoutes := [](Route){}
	for route, info := range gBotData.RouteInfo {
		if shouldDropRoute(route) {
			dropRoutes = append(dropRoutes, route)
			log.Println(fmt.Sprintf("[l] dropping route %s (%s)", route.Number, route.Date))
			continue
		}
		newInfo := fetchRouteInfo(route)
		if newInfo == info || !newInfo.Valid {
			continue
		}
		log.Println("[l] updated info for route " + route.Number + " (" + route.Date + "), sending notifications")
		for userID, active := range gBotData.RouteUsers[route] {
			if active {
				sendNotification(userID, route, info, newInfo)
			}
		}
		gBotData.RouteInfo[route] = newInfo
	}

	for _, route := range dropRoutes {
		for userID, active := range gBotData.RouteUsers[route] {
			if active {
				gBotData.UserRoutes[userID][route] = false
			}
		}
		delete(gBotData.RouteInfo, route)
		delete(gBotData.RouteUsers, route)
	}

	saveBotData()
}

func shouldDropRoute(route Route) bool {
	if !isValidDate(route.Date) {
		return true
	}
	hasUsers := false
	for _, active := range gBotData.RouteUsers[route] {
		if active {
			hasUsers = true
			break
		}
	}
	return !hasUsers
}
