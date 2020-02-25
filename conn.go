package wsutil

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

// Conn :
type Conn struct {
	conn           *websocket.Conn
	readBuff       []byte // a single json object can be transmitted via several packages
	ID             string
	Read           chan map[string]interface{}
	Write          chan map[string]interface{}
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
	c.readBuff = make([]byte, 0)
	uuidstring, _ := uuid.NewV4()
	c.ID = base64.RawURLEncoding.EncodeToString(uuidstring.Bytes())
	c.Read = make(chan map[string]interface{})
	c.Write = make(chan map[string]interface{})
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
	close(c.Read)
	close(c.Write)
	c.conn.Close()
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

		obj := make(map[string]interface{})
		err = json.Unmarshal(append(c.readBuff, message...), &obj)
		if err == nil {
			c.readBuff = make([]byte, 0)
			c.Read <- obj
			continue
		}

		obj2 := make(map[string]interface{})
		err = json.Unmarshal(message, &obj2)
		if err == nil {
			c.readBuff = make([]byte, 0)
			c.Read <- obj2
			continue
		}

		c.readBuff = append(c.readBuff, message...)
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
			b, err := json.Marshal(message)
			if err != nil {
				continue
			}
			c.conn.WriteMessage(websocket.TextMessage, b)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.dispose()
				return
			}
		}
	}
}
