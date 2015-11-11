package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethicatech/tinybiome/client"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"runtime"
	"time"
)

func main() {
	port := 3000
	d, e := websocket.Dial("ws://tinybio.me:4000", "", "http://server.go")
	if e != nil {
		log.Println("MASTER SERVER DOWN", e)
		d, e = websocket.Dial("ws://localhost:4000", "", "http://server.go")
		if e != nil {
			log.Panicln("LOCAL SERVER DOWN", e)
		}
	}
	writer := json.NewEncoder(d)
	go func() {
		for {
			time.Sleep(time.Second)
			if e := writer.Encode(map[string]interface{}{"meth": "ping"}); e != nil {
				break
			}
		}
	}()
	writer.Encode(map[string]interface{}{"meth": "addme", "port": port})
	runtime.SetBlockProfileRate(1)
	room := client.NewRoom()
	room.SetDimensions(5000, 5000)
	cli := client.NewServer()
	cli.AddRoom(room)
	log.Println("WEBSOCKETS STARTING")
	m := http.NewServeMux()
	m.HandleFunc("/", cli.Handler)
	add := fmt.Sprintf("0.0.0.0:%d", port)
	err := http.ListenAndServe(add, m)
	if err != nil {
		log.Println("ERROR", err)
	}
}
