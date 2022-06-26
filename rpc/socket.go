package rpc

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

type Socket struct {
	conn      *websocket.Conn
	ticker    *time.Ticker
	waitGroup *sync.WaitGroup

	rx chan []byte
	tx chan []byte

	C chan struct{}
}

func (cs Socket) readPump() {
	cs.waitGroup.Add(1)
	defer cs.waitGroup.Done()

	cs.conn.SetReadLimit(MaxMessageSize)
	cs.conn.SetReadDeadline(time.Now().Add(PongWait))
	cs.conn.SetPongHandler(func(string) error { cs.conn.SetReadDeadline(time.Now().Add(PongWait)); return nil })

	for {
		_, message, err := cs.conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println("Read failed: ", err)

			}
			cs.Close()
			return
		}

		cs.rx <- message
	}
}

func (cs Socket) writePump() {
	cs.waitGroup.Add(1)
	defer cs.waitGroup.Done()

	for {
		select {
		case message, ok := <-cs.tx:
			cs.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				cs.conn.WriteMessage(websocket.CloseMessage, []byte{})
				cs.Close()
				break
			}

			if err := cs.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println("Write to send request: ", err)
				cs.Close()
				break
			}

		case <-cs.ticker.C:
			cs.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := cs.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Println("Ping failed: ", err)
				cs.Close()
				break
			}
		}
	}
}

func (cs Socket) Close() {
	cs.conn.Close()
	cs.ticker.Stop()
	cs.C <- struct{}{}
}

func NewSocket(socketUrl url.URL, jar *cookiejar.Jar, rx chan []byte, tx chan []byte, wg *sync.WaitGroup) (*Socket, error) {
	log.Printf("Connecting to %s", socketUrl.String())

	var cloudDialer = &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
		Jar:              jar,
	}
	conn, _, err := cloudDialer.Dial(socketUrl.String(), nil)

	if err != nil {
		log.Println("Handshake failed:", err)
		return nil, err
	}
	log.Println("Connected")

	socket := Socket{
		conn:      conn,
		rx:        rx,
		tx:        tx,
		waitGroup: wg,
		ticker:    time.NewTicker(PingPeriod),
	}

	go socket.readPump()
	go socket.writePump()

	return &socket, nil
}
