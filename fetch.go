package main

import (
	"encoding/json"
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
	urlValues.Add("from", route.from)
	urlValues.Add("to", route.to)
	urlValues.Add("date", route.date)
	urlValues.Add("train_number", route.number)
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
		valid:          false,
		hasPlaces:      false,
		hasLowerPlaces: false,
	}

	// TODO: all valid car types here
	for _, carType := range [](int){3, 4} {
		mp := fetchJSON(formRouteURL(route, carType))
		if mp != nil {
			result.valid = true
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
					result.hasPlaces = true
				}
				lowerPlaces, ok := car.(map[string]interface{})["lowerPlaces"].(float64)
				if ok && lowerPlaces != 0 {
					result.hasLowerPlaces = true
				}
			}
		}
	}

	return result
}

func updateRoutesInfo() {
	for route, info := range gBotData.routeInfo {
		newInfo := fetchRouteInfo(route)
		if newInfo == info || !newInfo.valid {
			continue
		}
		log.Println("[l] updated info for route " + route.number + " (" + route.date + "), sending notifications")
		for userID, active := range gBotData.routeUsers[route] {
			if active {
				sendNotification(userID, route, info, newInfo)
			}
		}
		gBotMutex.Lock()
		gBotData.routeInfo[route] = newInfo
		gBotMutex.Unlock()
	}
}

func updateRoutesLoop() {
	for {
		updateRoutesInfo()
		time.Sleep(time.Minute)
	}
}
