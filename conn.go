package wsutil

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

// Conn :
type Conn struct {
	conn           *websocket.Conn
	readBuffer     []byte // a single json object can be transmitted via several packages
	ID             string
	Read           chan map[string]interface{}
	Write          chan map[string]interface{}
	WriteWait      time.Duration
	PongWait       time.Duration
	PingPeriod     time.Duration
	MaxMessageSize int64
}

// Upgrade : pass in nil for upgrader to use the default one
func Upgrade(w http.ResponseWriter, r *http.Request, upgrader *websocket.Upgrader) (*Conn, error) {
	if upgrader == nil {
		upgrader = &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	c := &Conn{}
	c.conn = conn
	c.readBuffer = make([]byte, 0)
	uuidstring, _ := uuid.NewV4()
	c.ID = base64.RawURLEncoding.EncodeToString(uuidstring.Bytes())
	c.Read = make(chan map[string]interface{})
	c.Write = make(chan map[string]interface{})
	c.WriteWait = 10 * time.Second
	c.PongWait = 60 * time.Second
	c.PingPeriod = (c.PongWait * 9) / 10
	c.MaxMessageSize = 512
	return c, nil
}

// Dial :
func Dial(addr string, header http.Header) (*Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(addr, header)
	if err != nil {
		return nil, err
	}
	c := &Conn{}
	c.conn = conn
	c.readBuffer = make([]byte, 0)
	uuidstring, _ := uuid.NewV4()
	c.ID = base64.RawURLEncoding.EncodeToString(uuidstring.Bytes())
	c.Read = make(chan map[string]interface{})
	c.Write = make(chan map[string]interface{})
	c.WriteWait = 10 * time.Second
	c.PongWait = 60 * time.Second
	c.PingPeriod = (c.PongWait * 9) / 10
	c.MaxMessageSize = 512
	return c, nil
}

// Start :
func (c *Conn) Start() {
	go c.readPump()
	go c.writePump()
}

// Close :
func (c *Conn) Close() {
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
			c.Close()
			return
		}
		if mt == websocket.BinaryMessage {
			continue
		}

		obj := make(map[string]interface{})
		err = json.Unmarshal(append(c.readBuffer, message...), &obj)
		if err == nil {
			c.readBuffer = make([]byte, 0)
			c.Read <- obj
			continue
		}

		obj2 := make(map[string]interface{})
		err = json.Unmarshal(message, &obj2)
		if err == nil {
			c.readBuffer = make([]byte, 0)
			c.Read <- obj2
			continue
		}

		c.readBuffer = append(c.readBuffer, message...)
	}
}

func (c *Conn) writePump() {
	ticker := time.NewTicker(c.PingPeriod)
	defer ticker.Stop()
	for {
		select {
		case message, ok := <-c.Write:
			if !ok {
				if c.conn != nil {
					c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				}
				c.Close()
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			b, err := json.Marshal(message)
			if err != nil {
				continue
			}
			c.conn.WriteMessage(websocket.TextMessage, b)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		}
	}
}
