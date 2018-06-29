package tinybiome

import (
	"log"
	_ "net/http/pprof"
	"os"
	"runtime"
)

func StartNode() {
	runtime.SetBlockProfileRate(1)

	var conf *ServerConfig
	confFile, e := os.Open("conf.json")

	if e == nil {
		conf, e = NewServerConfigFromReader(confFile)
		if e != nil {
			log.Println(e.Error())
			return
		}
	} else {
		log.Println("Using default config, no conf.json present")
		conf = NewServerConfigDefault()
	}

	server := NewServer(conf)
	server.Start()
}
