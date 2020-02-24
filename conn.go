package websocketconn

import (
	"encoding/base64"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/objx"
)

// Conn :
type Conn struct {
	ID             string
	conn           *websocket.Conn
	readBuff       string // a single json object can be transmitted via several packages
	Read           chan map[string]interface{}
	Write          chan []byte
	WriteWait      time.Duration
	PongWait       time.Duration
	PingPeriod     time.Duration
	MaxMessageSize int64
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// CreateConn :
func CreateConn(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	c := &Conn{}
	c.conn = conn
	uuidstring, _ := uuid.NewV4()
	c.ID = base64.RawURLEncoding.EncodeToString(uuidstring.Bytes())
	c.readBuff = ""
	c.Read = make(chan map[string]interface{})
	c.Write = make(chan []byte)
	c.WriteWait = 10 * time.Second
	c.PongWait = 60 * time.Second
	c.PingPeriod = (c.PongWait * 9) / 10
	c.MaxMessageSize = 512
	defer c.dispose()
	return c, nil
}

// Start :
func (c *Conn) Start() {
	go c.readPump()
	go c.writePump()
}

func (c *Conn) dispose() {
	if c.Read != nil {
		close(c.Read)
		c.Read = nil
	}
	if c.Write != nil {
		close(c.Write)
		c.Write = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Conn) readPump() {
	c.conn.SetReadLimit(c.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.PongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(c.PongWait)); return nil })
	for {
		mt, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			c.dispose()
			return
		}
		if mt == websocket.BinaryMessage {
			continue
		}

		m := c.readBuff + string(message)
		json, err := objx.FromJSON(m)
		if err == nil {
			c.readBuff = ""
			c.Read <- json
			continue
		}

		json2, err := objx.FromJSON(string(message))
		if err == nil { // dispose broken readBuff
			c.readBuff = ""
			c.Read <- json2
			continue
		}

		c.readBuff = m
	}
}

func (c *Conn) writePump() {
	ticker := time.NewTicker(c.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case message, ok := <-c.Write:
			c.conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.dispose()
				return
			}

			c.conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.dispose()
				return
			}
		}
	}
}
