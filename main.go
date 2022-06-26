package main

import (
	"fmt"
	"klipper-cloud-control-client/auth"
	"klipper-cloud-control-client/config"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"
)

const (
	ReconnectTimeout = time.Second * 5
)

func main() {
	config.LoadConfig("config.yaml")

	if config.GetConfig().Token == nil {

		auth := auth.DeviceAuth{}

		code, err := auth.GetDeviceCode()
		if err != nil {
			log.Fatal("Failed to get device code: ", err)
		}

		fmt.Println("Authorize printer using url ", code.GetUrl(), " and code ", code.GetCode())

		token, err := code.GetToken()
		if err != nil {
			log.Fatal("Failed to get device token: ", err)
		}

		err = config.StoreToken("config.yaml", token)
		if err != nil {
			log.Fatal("Failed to save token: ", err)
		}
	} else {
		token, err := auth.RefreshToken(*config.GetConfig().Token)
		if err != nil {
			log.Fatal("Failed to refresh device token: ", err)
		}

		err = config.StoreToken("config.yaml", token)
		if err != nil {
			log.Fatal("Failed to save token: ", err)
		}
	}

	jar, err := auth.DoAuth(config.GetConfig().Token)

	if err != nil {
		log.Fatal("Failed to check token: ", err)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	cloudUrl, err := url.Parse(config.GetConfig().GetUpstream() + "/printsocket")
	if err != nil {
		log.Fatal("Failed to parse hostname: ", err)
	}

	moonrakerUrl := url.URL{Scheme: "ws", Host: config.GetConfig().Moonraker, Path: "/websocket"}

	claimed := make(chan struct{}, 2)
	shutdown := make(chan struct{})

	wg := sync.WaitGroup{}

	running := true

	reconnect := time.NewTicker(ReconnectTimeout)
	reconnect.Stop()
	claimed <- struct{}{}

	for running {
		select {
		case _ = <-reconnect.C:
			claimed <- struct{}{}
			reconnect.Stop()
		case _ = <-claimed:
			log.Println("Creating connection")
			if _, err := NewProxy(moonrakerUrl, *cloudUrl, jar, claimed, shutdown, &wg); err != nil {
				log.Println("Failed to create proxy: ", err)

				reconnect.Reset(ReconnectTimeout)
			}
		case <-interrupt:
			running = false
			close(shutdown)
			break
		}
	}
	wg.Wait()
}
