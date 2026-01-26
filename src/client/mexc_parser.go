package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var TockenState = &models.SharedState{
	Mu:           sync.Mutex{},
	TickerStates: make(map[string]models.TickerState),
}
var httpClient = &http.Client{
	Timeout: 3 * time.Second,
}

func GetNextSplash(currentChange float64, lastTriggeredLevel float64) (models.SplashTier, bool) {
	var triggeredLevel models.SplashTier
	found := false

	if len(models.CurrentConfig.Tiers) == 0 {
		return triggeredLevel, false
	}

	for _, tier := range models.CurrentConfig.Tiers {
		if tier.Level <= 0 {
			continue
		}

		targetRate := tier.Level / 100.0
		if targetRate > lastTriggeredLevel && currentChange >= targetRate {
			if !found || tier.Level > triggeredLevel.Level {
				triggeredLevel = tier
				found = true
			}
		}
	}
	return triggeredLevel, found
}

func FetchAllFuturesTickers() ([]models.SplashData, error) {
	resp, err := httpClient.Get(models.FuturesRestAPI)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResponce models.Responce
	if err := json.Unmarshal(body, &apiResponce); err != nil {
		log.Printf("JSON Decode Error: %v", err)
		return nil, err
	}
	return apiResponce.Data, nil
}

func StartPolling(ctx context.Context) {
	models.AppCtx = ctx
	const postgresConnString = ""
	database.InitDatabase(postgresConnString)

	log.Println("Terminus Engine: Online")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		newTickers, err := FetchAllFuturesTickers()
		if err != nil {
			continue
		}

		TockenState.Mu.Lock()
		for _, t := range newTickers {
			state, exists := TockenState.TickerStates[t.Symbol]

			if !exists {
				TockenState.TickerStates[t.Symbol] = models.TickerState{
					WindowStartRef:   t,
					LatestTickerData: t,
					LastRefUpdate:    now,
				}
				continue
			}

			if !state.SplashTrigger && now.Sub(state.LastRefUpdate) >= models.Window {
				state.WindowStartRef = t
				state.LastRefUpdate = now
				state.LastTriggeredLevel = 0
			}

			state.LatestTickerData = t
			TockenState.TickerStates[t.Symbol] = state
		}
		TockenState.Mu.Unlock()

		CheckPrices(newTickers, now)
	}
}

func CheckPrices(newTickers []models.SplashData, now time.Time) {
	for _, ticker := range newTickers {
		TockenState.Mu.Lock()
		state, ok := TockenState.TickerStates[ticker.Symbol]
		if !ok {
			TockenState.Mu.Unlock()
			continue
		}

		ref := state.WindowStartRef
		if ref.LastPrice <= 0 || ticker.LastPrice <= 0 {
			state.LatestTickerData = ticker
			TockenState.TickerStates[ticker.Symbol] = state
			TockenState.Mu.Unlock()
			continue
		}

		lastChange := math.Abs(ticker.LastPrice-ref.LastPrice) / ref.LastPrice
		fairChange := math.Abs(ticker.FairPrice-ref.FairPrice) / ref.FairPrice
		maxChange := math.Max(lastChange, fairChange)

		tier, isTriggered := GetNextSplash(maxChange, state.LastTriggeredLevel)

		if isTriggered {
			if !state.SplashTrigger || tier.Level > (state.LastTriggeredLevel*100) {
				TockenState.Mu.Unlock()
				SplashHandle(ticker, tier, lastChange, fairChange, ref, now, state)
				continue
			}
		}

		state.LatestTickerData = ticker
		TockenState.TickerStates[ticker.Symbol] = state
		TockenState.Mu.Unlock()
	}
}

func SplashHandle(ticker models.SplashData, tier models.SplashTier, lpCh, fpCh float64, ref models.SplashData, refTime time.Time, state models.TickerState) {
	now := time.Now()
	basisGap := (math.Abs(ticker.LastPrice-ticker.FairPrice) / ticker.FairPrice) * 100

	speed := now.Sub(refTime).Seconds()
	if state.SplashTrigger && !state.TriggerTime.IsZero() {
		speed = now.Sub(state.TriggerTime).Seconds()
	}

	direction := "UP"
	if ticker.LastPrice < ref.LastPrice {
		direction = "DOWN"
	}

	targetLevelInt := int(math.Round(tier.Level))

	if state.SplashTrigger {
		lastLevelInt := int(math.Round(state.LastTriggeredLevel * 100))
		if direction == state.SplashDirection && targetLevelInt > lastLevelInt {
			total, wins, _ := database.GetContextStats(direction, targetLevelInt, ticker.Volume24, basisGap, tier.Window)
			prob := -1.0
			if total >= 3 {
				prob = math.Round((float64(wins) / float64(total)) * 100)
			}

			database.UpdateSplashLevel(state.SplashRecordID, targetLevelInt, ticker.LastPrice, ticker.FairPrice, ticker.Volume24, prob, tier.Window)

			state.LastTriggeredLevel = float64(targetLevelInt) / 100.0
			state.CurrentTimeWindow = tier.Window
			TockenState.Mu.Lock()
			TockenState.TickerStates[ticker.Symbol] = state
			TockenState.Mu.Unlock()

			sendWailsEvent(ticker, direction, tier, prob, basisGap, speed, ref, "ACTIVE")
		}
		return
	}

	total, wins, _ := database.GetContextStats(direction, targetLevelInt, ticker.Volume24, basisGap, tier.Window)
	prob := -1.0
	if total >= 3 {
		prob = math.Round((float64(wins) / float64(total)) * 100)
	}

	record := models.SplashRecord{
		Symbol:           ticker.Symbol,
		Direction:        direction,
		TriggerLevel:     targetLevelInt,
		RefLastPrice:     ref.LastPrice,
		RefFairPrice:     ref.FairPrice,
		TriggerLastPrice: ticker.LastPrice,
		TriggerFairPrice: ticker.FairPrice,
		TriggerTime:      now,
		Volume24h:        ticker.Volume24,
		LongProbability:  prob,
		TimeWindow:       tier.Window,
	}

	recordID, err := database.SaveSplashRecord(record, basisGap, speed)
	if err != nil {
		return
	}

	state.SplashRecordID = recordID
	state.LastTriggeredLevel = float64(targetLevelInt) / 100.0
	state.SplashTrigger = true
	state.TriggerTime = now
	state.SplashDirection = direction

	TockenState.Mu.Lock()
	TockenState.TickerStates[ticker.Symbol] = state
	TockenState.Mu.Unlock()

	sendWailsEvent(ticker, direction, tier, prob, basisGap, speed, ref, "ACTIVE")

	go TrackReturnBack(recordID, ticker.Symbol, ref.LastPrice, ref.FairPrice, now, direction, tier.Window)
}

func sendWailsEvent(ticker models.SplashData, dir string, tier models.SplashTier, prob, gap, spd float64, prev models.SplashData, status string) {
	if models.AppCtx == nil {
		return
	}
	runtime.EventsEmit(models.AppCtx, "splash:new", map[string]interface{}{
		"symbol":       ticker.Symbol,
		"exchange":     "MEXC",
		"direction":    dir,
		"level":        int(tier.Level),
		"activeWindow": tier.Window,
		"prob":         math.Round(prob),
		"refLast":      fmt.Sprintf("%.6f", prev.LastPrice),
		"refFair":      fmt.Sprintf("%.6f", prev.FairPrice),
		"lastPrice":    fmt.Sprintf("%.6f", ticker.LastPrice),
		"fairPrice":    fmt.Sprintf("%.6f", ticker.FairPrice),
		"gap":          fmt.Sprintf("%.2f", gap),
		"speed":        fmt.Sprintf("%.1f", spd),
		"volume":       ticker.Volume24,
		"timestamp":    time.Now().Format("15:04:05"),
		"status":       status,
	})
}
