package main

import (
	"flag"
	"github.com/ethicatech/tinybiome"
	"log"
	"strings"
	"sync"
)

func main() {
	flag.Parse()

	var all sync.WaitGroup
	for _, v := range flag.Args() {
		parts := strings.Split(v, ",")
		all.Add(1)
		switch parts[0] {
		case "node":
			go tinybiome.StartNode()
		case "master":
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
