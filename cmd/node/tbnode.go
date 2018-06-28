package main

import (
	"github.com/ethicatech/tinybiome/client"
	"log"
	_ "net/http/pprof"
	"os"
	"runtime"
)

func main() {
	runtime.SetBlockProfileRate(1)

	var conf *client.ServerConfig
	confFile, e := os.Open("conf.json")

	if e == nil {
		conf, e = client.NewServerConfigFromReader(confFile)
		if e != nil {
			log.Println(e.Error())
			return
		}
	} else {
		log.Println("Using default config, no conf.json present")
		conf = client.NewServerConfigDefault()
	}

	server := client.NewServer(conf)
	server.Start()
}
