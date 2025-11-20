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
	"splash-trading-bot/src/internal"
	"syscall"
	"time"
)

var previousPrices = make(map[string]internal.SplashData)

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

	for {
		select {
		case <-ticker.C:
			newTickers, err := FetchAllFuturesTickers()
			if err != nil {
				log.Printf("Polling failed: %v", err)
				return
			}

			CheckPrices(newTickers)

		case <-interrupt:
			log.Println("Got interrupt signal, stopping parser.")
			return
		}
	}
}

func CheckPrices(newTickers []internal.SplashData) {
	splashCount := 0
	for _, ticker := range newTickers {
		if prevTicker, ok := previousPrices[ticker.Symbol]; ok {

			if ticker.LastPrice == 0 || ticker.FairPrice == 0 {
				previousPrices[ticker.Symbol] = ticker
				continue
			}

			lastPriceChange := math.Abs(ticker.LastPrice-prevTicker.LastPrice) / prevTicker.LastPrice
			fairPriceChange := math.Abs(ticker.FairPrice-prevTicker.FairPrice) / prevTicker.FairPrice
			if fairPriceChange >= internal.SplashPercent && lastPriceChange >= internal.SplashPercent {
				splashCount++
				if ticker.FairPrice > prevTicker.FairPrice && ticker.LastPrice > prevTicker.LastPrice {
					log.Printf("SPLASH DETECTED")
					log.Printf("FAIR PRICE SPLASH: %s | Price moved +%.2f%% | Prev: %.6f | Current: %.6f | Volume24h: %v",
						ticker.Symbol, fairPriceChange*100, prevTicker.FairPrice, ticker.FairPrice, ticker.Volume24)
					log.Printf("LAST PRICE SPLASH: %s | Price moved +%.2f%% | Prev: %.6f | Current: %.6f | Volume24h: %v",
						ticker.Symbol, lastPriceChange*100, prevTicker.LastPrice, ticker.LastPrice, ticker.Volume24)
				}
				if ticker.FairPrice < prevTicker.FairPrice {
					log.Printf("FAIR PRICE SPLASH DETECTED: %s | Price moved -%.2f%% | Prev: %.6f | Current: %.6f | Volume24h: %v",
						ticker.Symbol, fairPriceChange*100, prevTicker.FairPrice, ticker.LastPrice, ticker.Volume24)
					log.Printf("LAST PRICE SPLASH: %s | Price moved +%.2f%% | Prev: %.6f | Current: %.6f | Volume24h: %v",
						ticker.Symbol, lastPriceChange*100, prevTicker.LastPrice, ticker.LastPrice, ticker.Volume24)
				}
			}

		}

		previousPrices[ticker.Symbol] = ticker
	}

	log.Printf("Polling successful. Checked %d symbols. Found %d splashes.", len(newTickers), splashCount)
}
