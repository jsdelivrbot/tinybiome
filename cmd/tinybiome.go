package main

import (
	"flag"
	"github.com/ethicatech/tinybiome"
	"log"
	"strings"
	"sync"
)

func main() {
	defer log.Println("exiting")
	flag.Parse()

	var all sync.WaitGroup
	for _, v := range flag.Args() {
		parts := strings.Split(v, ",")
		log.Println("running service", v)
		all.Add(1)
		switch parts[0] {
		case "node":
			go tinybiome.StartNode()
		case "master":
			go tinybiome.ListenForClients()
			all.Add(1)
			go tinybiome.ListenForNodes()
		case "files":
			go tinybiome.ServeStaticFiles()
		default:
			log.Println("unknown command", v)
			return
		}
	}

	all.Wait()
}
