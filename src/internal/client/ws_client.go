package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"splash-trading-bot/src/internal"
	"splash-trading-bot/src/internal/processor"
	"time"

	"github.com/gorilla/websocket"
)

func GetSymbols() ([]string, error) {
	log.Printf("Symbol list request")

	resp, err := http.Get(internal.FuturesRestAPI)
	if err != nil {
		return nil, fmt.Errorf("Cant reach API through http %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad API Request, status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Cant read responce body: %w", err)
	}

	var apiResponce internal.Responce
	if err := json.Unmarshal(body, &apiResponce); err != nil {
		return nil, fmt.Errorf("Error decoding JSON responce: %w", err)
	}

	// if apiResponce.Code != 200 {
	// 	// Добавим проверку на null, если Msg является *string
	// 	errMsg := fmt.Sprintf("API returned error code: %d", apiResponce.Code)
	// 	if apiResponce.Msg != "" {
	// 		errMsg += fmt.Sprintf(" Message: %s", apiResponce.Msg)
	// 	}
	// 	return nil, fmt.Errorf(errMsg)
	// }

	symbols := apiResponce.Data
	log.Printf("%v Symbols got", symbols)
	return symbols, nil
}

func WsClient() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	allSymbols, err := GetSymbols()
	if err != nil {
		log.Fatalf("Error in symbol request: %v", err)
	}

	var subTopics []string

	for _, symbol := range allSymbols {
		subTopics = append(subTopics, "deal."+symbol)
	}

	u, _ := url.Parse(internal.MexcWsURl)
	log.Printf("Connecting to Mexc websocket %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Cant connect to Mexc WebSocket")
	}
	defer c.Close()

	done := make(chan struct{})
	subMsg := internal.SubMessage{
		Method: "SUBSCRIPTION",
		Params: subTopics,
	}
	if err := c.WriteJSON(subMsg); err != nil {
		log.Fatal("Error sending subscription: %w", err)
	}
	log.Printf("Subscription sended on %d topics", len(subTopics))

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("Error in reading/writing connection", err)
				return
			}
			processor.WsMsgProcess(message)

		}
	}()

	select {
	case <-done:
	case <-interrupt:
		log.Printf("Got interrupt signal, closing")
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			log.Println("Error writing closing:", err)
			return
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
	log.Println("Parser stopped")
}
