package client

import (
	"fmt"
	"log"
	"math"
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TrackReturnBack(
	recordID int64,
	symbol string,
	refLastPrice float64,
	refFairPrice float64,
	triggerTime time.Time,
	direction string,
	userWindowMin int,
) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	warmup := 0
	maxDeviation := 0.0

	for range ticker.C {
		if warmup < 2 {
			warmup++
			continue
		}
		TockenState.Mu.Lock()
		state, ok := TockenState.TickerStates[symbol]
		TockenState.Mu.Unlock()

		if !ok || !state.SplashTrigger || state.SplashRecordID != recordID {
			return
		}
		currentLevel := state.LastTriggeredLevel

		maxReturnWindow := time.Duration(userWindowMin) * time.Minute

		tolerance := dynamicTolerance(currentLevel)

		timeSinceTrigger := time.Since(triggerTime)
		if timeSinceTrigger > maxReturnWindow {
			log.Printf("TIMEOUT: %s exceeded user window of %d min", symbol, userWindowMin)
			if models.AppCtx != nil {
				runtime.EventsEmit(models.AppCtx, "splash:new", map[string]interface{}{
					"symbol": symbol,
					"status": "TIMEOUT",
				})
			}
			SaveReturnBackRecord(recordID, false, timeSinceTrigger, maxDeviation)
			resetTickerState(symbol)
			return
		}
		currentData := state.LatestTickerData
		if currentData.LastPrice == 0 || currentData.FairPrice == 0 {
			continue
		}

		lastPriceChangeToRef := math.Abs(currentData.LastPrice-refLastPrice) / refLastPrice
		fairPriceChangeToRef := math.Abs(currentData.FairPrice-refFairPrice) / refFairPrice
		currentDeviation := math.Max(lastPriceChangeToRef, fairPriceChangeToRef)

		if currentDeviation > maxDeviation {
			maxDeviation = currentDeviation
		}

		if currentDeviation <= tolerance {
			timeToReturn := time.Since(triggerTime)
			log.Printf("PRICE RETURNED: %s | LEVEL: %.0f%%", symbol, currentLevel*100)

			if models.AppCtx != nil {
				runtime.EventsEmit(models.AppCtx, "splash:new", map[string]interface{}{
					"symbol":     symbol,
					"status":     "RETURNED",
					"returnTime": fmt.Sprintf("%.2f", timeToReturn.Seconds()),
					"lastPrice":  fmt.Sprintf("%.6f", currentData.LastPrice),
					"fairPrice":  fmt.Sprintf("%.6f", currentData.FairPrice),
				})
			}
			SaveReturnBackRecord(recordID, true, timeToReturn, maxDeviation)
			resetTickerState(symbol)
			return
		}

	}
}

func resetTickerState(symbol string) {
	TockenState.Mu.Lock()
	defer TockenState.Mu.Unlock()

	state, ok := TockenState.TickerStates[symbol]
	if ok {
		state.SplashTrigger = false
		state.TriggerTime = time.Time{}
		state.SplashDirection = ""
		state.LastTriggeredLevel = 0.0
		state.SplashRecordID = 0

		TockenState.TickerStates[symbol] = state
	}
}

func SaveReturnBackRecord(recordID int64, returned bool, returnTime time.Duration, maxDeviation float64) {
	record, err := database.GetSplashRecordByID(recordID)
	if err != nil {
		log.Printf("Error saving return back info for record ID %d: %v", recordID, err)
		return
	}
	record.Returned = true
	record.ReturnTime = returnTime
	record.MaxDeviation = maxDeviation

	err = database.UpdateSplashRecord(record)
	if err != nil {
		log.Printf("Error updating splash record ID %d: %v", recordID, err)
		return
	}
}

func dynamicTolerance(level float64) float64 {
	return models.ReturnTolerance + (level / 100 * 0.1)
}
