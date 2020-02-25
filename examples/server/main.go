package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/aki-xavier/wsutil"
)

func serveWs(w http.ResponseWriter, r *http.Request) {
	conn, err := wsutil.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	conn.Start()
	for {
		message, ok := <-conn.Read
		if !ok {
			break
		}
		log.Println(message)
		obj := make(map[string]interface{})
		err := json.Unmarshal([]byte(`{"id": "aki", "status": "ok"}`), &obj)
		if err != nil {
			panic(err)
		}
		conn.Write <- obj
		time.Sleep(5 * time.Second)
		break
	}
	conn.Close()
	log.Println("ending conn")
}

func main() {
	http.HandleFunc("/ws", serveWs)
	log.Println("start listening at :8848")
	err := http.ListenAndServe("localhost:8848", nil)
	if err != nil {
		panic(err)
	}
}
