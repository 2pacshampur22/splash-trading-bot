package main

import (
	"log"
	"splash-trading-bot/src/client"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting Splash-parser")
	client.StartPolling()
	log.Println("Parser Stopped")
}
