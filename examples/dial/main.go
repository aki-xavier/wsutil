package main

import (
	"encoding/json"
	"log"

	"github.com/aki-xavier/wsutil"
)

func main() {
	wsutil.Debug = true
	conn, err := wsutil.Dial("ws://localhost:8848/ws", nil)
	if err != nil {
		panic(err)
	}
	conn.Start()

	obj := make(map[string]interface{})
	err = json.Unmarshal([]byte(`{"hello":"world"}`), &obj)
	if err != nil {
		panic(err)
	}

	conn.Write <- obj

	for {
		json, ok := <-conn.Read
		if !ok {
			break
		}
		log.Println(json)
	}
}
