package main

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"
)

const (
	MaxMessageSize = 1024 * 1024
	MaxBufferSize  = 1024 * 1024
	ChannelSize    = 256
	PingPeriod     = 5 * time.Second
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
)

type Proxy struct {
	printerConn *websocket.Conn
	cloudConn   *websocket.Conn

	fromPrinter   chan []byte
	fromCloud     chan []byte
	used          chan struct{}
	shutdown      chan struct{}
	waitGroup     *sync.WaitGroup
	printerTicker *time.Ticker
	cloudTicker   *time.Ticker
}

func (proxy Proxy) close() {
	proxy.printerTicker.Stop()
	proxy.cloudTicker.Stop()
	proxy.cloudConn.Close()
	proxy.printerConn.Close()
}

func (proxy Proxy) readPump(socket *websocket.Conn, data chan []byte) {
	proxy.waitGroup.Add(1)
	defer proxy.waitGroup.Done()
	defer close(data)

	// Only first message from cloud claims connection
	firstMessage := socket == proxy.cloudConn

	socket.SetReadLimit(MaxMessageSize)
	socket.SetReadDeadline(time.Now().Add(PongWait))
	socket.SetPongHandler(func(string) error { socket.SetReadDeadline(time.Now().Add(PongWait)); return nil })

	for {
		_, message, err := socket.ReadMessage()

		if firstMessage {
			firstMessage = false
			proxy.used <- struct{}{}
			log.Println("Connection claimed")
		}

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if socket == proxy.printerConn {
					log.Println("Read from printer failed: ", err)
				} else if socket == proxy.cloudConn {
					log.Println("Read from cloud failed: ", err)
				} else {
					panic("Unknown socket")
				}
			}
			proxy.close()
			break
		}

		data <- message
	}
}

func (proxy Proxy) writePump(socket *websocket.Conn, data chan []byte, ticker *time.Ticker) {
	proxy.waitGroup.Add(1)
	defer proxy.waitGroup.Done()

	for {
		select {
		case message, ok := <-data:
			socket.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				socket.WriteMessage(websocket.CloseMessage, []byte{})
				proxy.close()
				break
			}

			if err := socket.WriteMessage(websocket.TextMessage, message); err != nil {
				if socket == proxy.printerConn {
					log.Println("Write to printer failed: ", err)
				} else if socket == proxy.cloudConn {
					log.Println("Write to cloud failed: ", err)
				} else {
					panic("Unknown socket")
				}
				proxy.close()
				break
			}

		case <-ticker.C:
			socket.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := socket.WriteMessage(websocket.PingMessage, nil); err != nil {
				if socket == proxy.printerConn {
					log.Println("Ping printer failed: ", err)
				} else if socket == proxy.cloudConn {
					log.Println("Ping cloud failed: ", err)
				} else {
					panic("Unknown socket")
				}
				proxy.close()
				break
			}
		case <-proxy.shutdown:
			proxy.close()
			break
		}
	}

}

func NewProxy(printerUrl url.URL, cloudUrl url.URL, jar *cookiejar.Jar, used chan struct{}, shutdown chan struct{}, wg *sync.WaitGroup) (*Proxy, error) {
	log.Printf("connecting to cloud %s", cloudUrl.String())

	var cloudDialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              jar,
	}
	cloudSocket, _, err := cloudDialer.Dial(cloudUrl.String(), nil)

	if err != nil {
		log.Println("Handshake failed:", err)
		return nil, err
	}
	log.Printf("Connected")

	log.Printf("connecting to printer %s", printerUrl.String())
	printerSocket, _, err := websocket.DefaultDialer.Dial(printerUrl.String(), nil)

	if err != nil {
		log.Println("Handshake failed:", err)
		return nil, err
	}

	log.Printf("Connected")

	proxy := Proxy{
		printerConn:   printerSocket,
		cloudConn:     cloudSocket,
		fromPrinter:   make(chan []byte, ChannelSize),
		fromCloud:     make(chan []byte, ChannelSize),
		used:          used,
		shutdown:      shutdown,
		printerTicker: time.NewTicker(PingPeriod),
		cloudTicker:   time.NewTicker(PingPeriod),
		waitGroup:     wg,
	}

	go proxy.readPump(proxy.printerConn, proxy.fromPrinter)
	go proxy.readPump(proxy.cloudConn, proxy.fromCloud)
	go proxy.writePump(proxy.printerConn, proxy.fromCloud, proxy.printerTicker)
	go proxy.writePump(proxy.cloudConn, proxy.fromPrinter, proxy.cloudTicker)

	return &proxy, nil
}
