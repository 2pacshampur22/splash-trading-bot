package client

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	internal "splash-trading-bot/lib/models"
	"syscall"
	"time"
)

var tockenState = make(map[string]internal.TickerState)

func GetNextSplash(currentChange float64, currentLevel float64) float64 {
	for _, level := range internal.SplashLevels {
		if level > currentLevel && currentChange >= level {
			return level
		}
	}
	return 0
}

func FetchAllFuturesTickers() ([]internal.SplashData, error) {
	resp, err := http.Get(internal.FuturesRestAPI)
	if err != nil {
		return nil, fmt.Errorf("cant reach API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad API Request, status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cant read responce body: %w", err)
	}

	var apiResponce internal.Responce
	if err := json.Unmarshal(body, &apiResponce); err != nil {
		log.Printf("Error decoding JSON: %v. Raw Body: %s", err, string(body))
		return nil, fmt.Errorf("error decoding JSON responce: %w", err)
	}

	if !apiResponce.Success || apiResponce.Code != 0 {
		return nil, fmt.Errorf("API returned error code: %d, success: %t", apiResponce.Code, apiResponce.Success)
	}

	return apiResponce.Data, nil
}

func StartPolling() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Starting REST Polling...")

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var referenceTime time.Time

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			newTickers, err := FetchAllFuturesTickers()
			if err != nil {
				log.Printf("Polling failed: %v", err)
				return
			}

			if tockenState == nil || now.Sub(referenceTime) >= internal.Window {
				newTockenState := make(map[string]internal.TickerState)
				for _, ticker := range newTickers {
					newTockenState[ticker.Symbol] = internal.TickerState{
						Reference:          ticker,
						LastTriggeredLevel: 0,
					}
				}
				tockenState = newTockenState
				referenceTime = now
				log.Println("Reference prices updated")
				continue
			}

			CheckPrices(newTickers, referenceTime)

		case <-interrupt:
			log.Println("Got interrupt signal, stopping parser.")
			return
		}
	}
}

func CheckPrices(newTickers []internal.SplashData, referenceTime time.Time) {
	splashCount := 0
	var timeAlert time.Time
	var direction string
	for _, ticker := range newTickers {

		state, ok := tockenState[ticker.Symbol]
		if !ok {
			continue
		}
		previousPrices := state.Reference
		if ticker.LastPrice == 0 || ticker.FairPrice == 0 {
			continue
		}
		lastPriceChangeRef := math.Abs(ticker.LastPrice-previousPrices.LastPrice) / previousPrices.LastPrice
		fairPriceChangeRef := math.Abs(ticker.FairPrice-previousPrices.FairPrice) / previousPrices.FairPrice

		maxChangeAlert := math.Max(lastPriceChangeRef, fairPriceChangeRef)
		nextLevel := GetNextSplash(maxChangeAlert, state.LastTriggeredLevel)
		if nextLevel > 0 {

			if math.Abs(lastPriceChangeRef-fairPriceChangeRef)*100 < 0.5 {
				timeAlert = time.Now()
				if ticker.LastPrice < lastPriceChangeRef && ticker.FairPrice < fairPriceChangeRef {
					direction = "DOWN"
				} else {
					direction = "UP"
				}
				splashCount++
				log.Printf("------------------------------------------")
				log.Printf("SPLASH DETECTED: %s | Level: %.0f%% %s", ticker.Symbol, nextLevel*100, direction)
				log.Printf("Total in 3m:")
				log.Printf("Last Price Change:%.2f%% | Reference Last Price: %.6f | Now last price: %.6f", lastPriceChangeRef*100, previousPrices.LastPrice, ticker.LastPrice)
				log.Printf("Fair Price Change:%.2f%% | Reference Fair Price: %.6f | Now fair price: %.6f", fairPriceChangeRef*100, previousPrices.FairPrice, ticker.FairPrice)
				log.Printf("Volume24h: %v", ticker.Volume24)
				if timeAlert.Sub(referenceTime).Minutes() > 1 {
					log.Printf("Splash time: %.2f min", timeAlert.Sub(referenceTime).Minutes())
				} else {
					log.Printf("Splash time: %.2f sec", timeAlert.Sub(referenceTime).Seconds())
				}
				log.Printf("------------------------------------------")

				state.LastTriggeredLevel = nextLevel
				tockenState[ticker.Symbol] = state
			}
		}

	}

	log.Printf("Polling successful. Checked %d symbols. Found %d splashes.", len(newTickers), splashCount)
}
