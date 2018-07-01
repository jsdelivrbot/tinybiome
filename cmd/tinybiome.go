package main

import (
	"flag"
	"github.com/ethicatech/tinybiome"
	"log"
	_ "net/http/pprof"
	"os"
)

var confFile = flag.String("conf", "default.yml", "configuration yaml file")

func main() {
	defer log.Println("exiting")
	flag.Parse()

	log.Println("loading config at", *confFile)
	config, err := tinybiome.ConfigFromFile(*confFile)
	if err != nil {
		log.Println("couldn't load config file:", err.Error())
		os.Exit(1)
		return
	}
	if err := config.RunAndWait(); err != nil {
		log.Println("quitting from err:", err.Error())
		os.Exit(2)
		return
	}

	log.Println("quitting happily")
	return
}
