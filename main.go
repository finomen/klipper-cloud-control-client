package main

import (
	"fmt"
	"klipper-cloud-control-client/auth"
	"klipper-cloud-control-client/config"
	"klipper-cloud-control-client/rpc"
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

	cloudUrl, err := url.Parse(config.GetConfig().GetUpstream())
	if err != nil {
		log.Fatal("Failed to parse hostname: ", err)
	}

	moonrakerUrl, err := url.Parse(config.GetConfig().MoonrakerSocket)
	if err != nil {
		log.Fatal("Failed to parse moonraker url: ", err)
	}

	cloudRx := make(chan []byte, rpc.ChannelSize)
	cloudTx := make(chan []byte, rpc.ChannelSize)
	printerRx := make(chan []byte, rpc.ChannelSize)
	printerTx := make(chan []byte, rpc.ChannelSize)

	wg := &sync.WaitGroup{}

	var cloudSocket *rpc.Socket
	var printerSocket *rpc.Socket

	_ = rpc.NewBridge(
		cloudRx,
		cloudTx,
		printerRx,
		printerTx, jar)

	cloudReconnect := time.NewTicker(3)
	printerReconnect := time.NewTicker(3)

	for {
		select {
		case _ = <-cloudReconnect.C:
			cloudReconnect.Stop()
			cloudSocket, err = rpc.NewSocket(*cloudUrl, jar, cloudRx, cloudTx, wg)
			if err != nil {
				log.Println("Failed to connect to cloud", err)
				cloudReconnect.Reset(ReconnectTimeout)
				continue
			}
			log.Println("Connected to cloud")

			go func() {
				<-cloudSocket.C
				log.Println("Reconnecting to cloud")
				cloudReconnect.Reset(ReconnectTimeout)
			}()
		case _ = <-printerReconnect.C:
			printerReconnect.Stop()
			printerSocket, err = rpc.NewSocket(*moonrakerUrl, jar, printerRx, printerTx, wg)
			if err != nil {
				log.Println("Failed to connect to printer", err)
				continue
			}
			printerReconnect.Stop()
			log.Println("Connected to printer")
			go func() {
				<-printerSocket.C
				log.Println("Reconnecting to printer")
				printerReconnect.Reset(ReconnectTimeout)
			}()
		case <-interrupt:
			break
		}
	}

}
