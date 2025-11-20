package main

import (
	"log"
	"splash-trading-bot/src/internal/client"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Starting Splash-parser")
	client.WsClient()
	log.Println("Parser Stopped")

}
