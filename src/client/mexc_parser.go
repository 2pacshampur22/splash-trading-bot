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
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"sync"
	"syscall"
	"time"
)

var TockenState = &models.SharedState{
	Mu:           sync.Mutex{},
	TickerStates: make(map[string]models.TickerState),
}

func GetNextSplash(currentChange float64, lastTriggeredLevel float64) float64 {
	maxLevel := 0.0
	for _, level := range models.SplashLevels {
		if level > lastTriggeredLevel && currentChange >= level {
			maxLevel = level
		}
	}
	return maxLevel
}

func FetchAllFuturesTickers() ([]models.SplashData, error) {
	resp, err := http.Get(models.FuturesRestAPI)
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

	var apiResponce models.Responce
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
	const postgresConnString = "host=localhost port=5432 user=postgres password=10072005Egor dbname=splashtradingbot sslmode=disable"
	err := database.InitDatabase(postgresConnString)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Starting REST Polling...")

	ticker := time.NewTicker(50 * time.Millisecond)
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
			TockenState.Mu.Lock()

			if TockenState == nil || now.Sub(referenceTime) >= models.Window {
				newTockenState := make(map[string]models.TickerState)
				for _, ticker := range newTickers {
					if ticker.LastPrice == 0 || ticker.FairPrice == 0 {
						continue
					}
					newTockenState[ticker.Symbol] = models.TickerState{
						WindowStartRef:     ticker,
						LatestTickerData:   ticker,
						LastTriggeredLevel: 0,
					}
				}
				TockenState.TickerStates = newTockenState
				referenceTime = now
				log.Println("Reference prices updated")

				TockenState.Mu.Unlock()
				continue
			}
			TockenState.Mu.Unlock()

			CheckPrices(newTickers, referenceTime)

		case <-interrupt:
			log.Println("Got interrupt signal, stopping parser.")
			return
		}
	}
}

func CheckPrices(newTickers []models.SplashData, referenceTime time.Time) {
	for _, ticker := range newTickers {
		TockenState.Mu.Lock()
		state, ok := TockenState.TickerStates[ticker.Symbol]

		if !ok {
			TockenState.Mu.Unlock()
			continue
		}

		previousPrices := state.WindowStartRef
		state.LatestTickerData = ticker
		TockenState.TickerStates[ticker.Symbol] = state
		if previousPrices.LastPrice == 0 || previousPrices.FairPrice == 0 {
			state.LatestTickerData = ticker
			TockenState.TickerStates[ticker.Symbol] = state
			TockenState.Mu.Unlock()
			continue
		}
		if ticker.LastPrice == 0 || ticker.FairPrice == 0 {
			state.LatestTickerData = ticker
			TockenState.TickerStates[ticker.Symbol] = state
			TockenState.Mu.Unlock()
			continue
		}
		lastPriceChangeRef := math.Abs(ticker.LastPrice-previousPrices.LastPrice) / previousPrices.LastPrice
		fairPriceChangeRef := math.Abs(ticker.FairPrice-previousPrices.FairPrice) / previousPrices.FairPrice
		maxChangeAlert := math.Max(lastPriceChangeRef, fairPriceChangeRef)

		nextLevel := GetNextSplash(maxChangeAlert, state.LastTriggeredLevel)
		if nextLevel > 0 {
			TockenState.Mu.Unlock()
			levelRound := math.Round(nextLevel * 100)
			valid := false
			switch {
			case levelRound == 1 || levelRound == 3 || levelRound == 5:
				valid = true
			case (levelRound == 12 || levelRound == 24) && math.Abs(lastPriceChangeRef-fairPriceChangeRef)*100 < 2.0:
				valid = true
			case (levelRound == 48) && math.Abs(lastPriceChangeRef-fairPriceChangeRef)*100 < 10.0:
				valid = true
			case (levelRound == 96) && math.Abs(lastPriceChangeRef-fairPriceChangeRef)*100 < 15.0:
				valid = true
			}
			if valid {
				SplashHandle(ticker, nextLevel, lastPriceChangeRef, fairPriceChangeRef, previousPrices, referenceTime, state)
				continue
			}
		}

		state.LatestTickerData = ticker
		TockenState.TickerStates[ticker.Symbol] = state
		TockenState.Mu.Unlock()

	}

}

func SplashHandle(ticker models.SplashData, nextLevel float64, lastPriceChangeRef float64, fairPriceChangeRef float64, previousPrices models.SplashData, referenceTime time.Time, state models.TickerState) {
	defer TockenState.Mu.Unlock()

	now := time.Now()
	basisGap := (math.Abs(ticker.LastPrice-ticker.FairPrice) / ticker.FairPrice) * 100
	var speedSeconds float64
	if state.SplashTrigger && !state.TriggerTime.IsZero() {
		speedSeconds = now.Sub(state.TriggerTime).Seconds()
	} else {
		speedSeconds = now.Sub(referenceTime).Seconds()
	}
	var direction string
	if ticker.LastPrice < previousPrices.LastPrice && ticker.FairPrice < previousPrices.FairPrice {
		direction = "DOWN"
	} else {
		direction = "UP"
	}

	total, wins, err := database.GetContextStats(direction, int(nextLevel*100), ticker.Volume24, basisGap)
	probability := -1.0
	if err == nil && total >= 3 {
		probability = (float64(wins) / float64(total)) * 100
	}

	log.Printf("------------------------------------------")
	log.Printf("SPLASH DETECTED: %s | Level: %.0f%% %s", ticker.Symbol, nextLevel*100, direction)
	log.Printf("Total in 3m:")
	log.Printf("Last Price Change:%.2f%% | Reference Last Price: %.6f | Now last price: %.6f", lastPriceChangeRef*100, previousPrices.LastPrice, ticker.LastPrice)
	log.Printf("Fair Price Change:%.2f%% | Reference Fair Price: %.6f | Now fair price: %.6f", fairPriceChangeRef*100, previousPrices.FairPrice, ticker.FairPrice)
	log.Printf("Volume24h: %v", ticker.Volume24)
	log.Printf("Gap: %.2f%% | Speed: %.1f sec", basisGap, speedSeconds)
	if speedSeconds > 1 {
		log.Printf("Splash time: %.2f min", speedSeconds/60)
	} else {
		log.Printf("Splash time: %.2f sec", speedSeconds)
	}
	log.Printf("------------------------------------------")

	TockenState.Mu.Lock()

	if state.SplashTrigger {
		if direction == "UP" && nextLevel > state.LastTriggeredLevel {
			err := database.UpdateSplashLevel(state.SplashRecordID, int(nextLevel*100), ticker.LastPrice, ticker.FairPrice, ticker.Volume24, probability)
			if err != nil {
				log.Printf("Error updating splash record in database: %v", err)
				return
			}
			state.LastTriggeredLevel = nextLevel
			TockenState.TickerStates[ticker.Symbol] = state
			log.Printf("%s UP PROGRESSION: Level %.0f%% | LAST PRICE: %.6f | FAIR PRICE %.6f | Volume: %d", ticker.Symbol, nextLevel*100, ticker.LastPrice, ticker.FairPrice, ticker.Volume24)
			log.Printf("Win probability: %.0f%% | Wins: %d | Total: %d", probability, wins, total)
			return
		} else if direction == "DOWN" && nextLevel > state.LastTriggeredLevel {
			err := database.UpdateSplashLevel(state.SplashRecordID, int(nextLevel*100), ticker.LastPrice, ticker.FairPrice, ticker.Volume24, probability)
			if err != nil {
				log.Printf("Error updating splash record in database: %v", err)
				return
			}
			state.LastTriggeredLevel = nextLevel
			TockenState.TickerStates[ticker.Symbol] = state
			log.Printf("%s DOWN PROGRESSION: Level %.0f%% | LAST PRICE: %.6f | FAIR PRICE %.6f | Volume: %d", ticker.Symbol, nextLevel*100, ticker.LastPrice, ticker.FairPrice, ticker.Volume24)
			log.Printf("Win probability: %.0f%% | Wins: %d | Total: %d", probability, wins, total)
			return
		}
		return
	}
	record := models.SplashRecord{
		Symbol:           ticker.Symbol,
		Direction:        direction,
		TriggerLevel:     int(nextLevel * 100),
		RefLastPrice:     previousPrices.LastPrice,
		RefFairPrice:     previousPrices.FairPrice,
		TriggerLastPrice: ticker.LastPrice,
		TriggerFairPrice: ticker.FairPrice,
		TriggerTime:      now,
		Volume24h:        ticker.Volume24,
		LongProbability:  probability,
	}

	recordID, err := database.SaveSplashRecord(record, basisGap, speedSeconds)
	if err != nil {
		log.Printf("[%s] DB SAVE ERROR: %v", ticker.Symbol, err)
		state.LastTriggeredLevel = nextLevel
		state.SplashTrigger = true
		TockenState.TickerStates[ticker.Symbol] = state
		return
	}
	state.SplashRecordID = recordID
	state.LastTriggeredLevel = nextLevel
	state.SplashTrigger = true
	state.TriggerTime = now
	state.SplashDirection = direction
	state.SplashRecordID = recordID

	TockenState.TickerStates[ticker.Symbol] = state
	go TrackReturnBack(
		recordID,
		ticker.Symbol,
		previousPrices.LastPrice,
		previousPrices.FairPrice,
		state.TriggerTime,
		state.SplashDirection,
	)

}
