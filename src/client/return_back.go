package client

import (
	"log"
	"math"
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"time"
)

func TrackReturnBack(
	recordID int64,
	symbol string,
	refLastPrice float64,
	refFairPrice float64,
	triggerTime time.Time,
	direction string,
) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	maxDeviation := 0.0

	for range ticker.C {

		TockenState.Mu.Lock()
		state, ok := TockenState.TickerStates[symbol]
		TockenState.Mu.Unlock()

		if !ok || !state.SplashTrigger || state.SplashRecordID != recordID {
			return
		}

		timeSinceTrigger := time.Since(triggerTime)
		if timeSinceTrigger > models.MaxReturnWindow {
			log.Printf("---------------------------------------------------------------")
			log.Printf("MAX RETURN WINDOW EXCEEDED: %s | LEVEL: %.0f%% | TIME SINCE TRIGGER: %.2f min",
				symbol, state.LastTriggeredLevel*100, timeSinceTrigger.Minutes())
			log.Printf("NOW LAST PRICE:  %.6f | NOW FAIR PRICE:  %.6f | REFERENCE PRICES: FAIR:  %.6f | LAST:  %.6f",
				state.LatestTickerData.LastPrice, state.LatestTickerData.FairPrice, refFairPrice, refLastPrice)
			log.Printf("---------------------------------------------------------------")
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

		if currentDeviation <= models.ReturnTolerance {
			timeToReturn := time.Since(triggerTime)

			TockenState.Mu.Lock()

			state.SplashTrigger = false
			state.TriggerTime = time.Time{}
			state.SplashDirection = ""

			TockenState.TickerStates[symbol] = state
			TockenState.Mu.Unlock()
			if timeToReturn.Minutes() >= 1 {
				log.Printf("---------------------------------------------------------------")
				log.Printf("PRICE RETURNED: %s | LEVEL: %.0f%% | RETURN BACK TIME: %.2f min |",
					symbol, state.LastTriggeredLevel*100, timeToReturn.Minutes())
				log.Printf("NOW LAST PRICE:  %.6f | NOW FAIR PRICE:  %.6f | REFERENCE PRICES: FAIR:  %.6f | LAST:  %.6f",
					currentData.LastPrice, currentData.FairPrice, refFairPrice, refLastPrice)
				log.Printf("---------------------------------------------------------------")
				SaveReturnBackRecord(recordID, timeToReturn, maxDeviation, lastPriceChangeToRef, fairPriceChangeToRef)
				resetTickerState(symbol)
				return
			} else {
				log.Printf("---------------------------------------------------------------")
				log.Printf("PRICE RETURNED: %s | LEVEL: %.0f%% | RETURN BACK TIME: %.2f sec",
					symbol, state.LastTriggeredLevel*100, timeToReturn.Seconds())
				log.Printf("NOW LAST PRICE:  %.6f | NOW FAIR PRICE:  %.6f | REFERENCE PRICES: FAIR:  %.6f | LAST:  %.6f",
					currentData.LastPrice, currentData.FairPrice, refFairPrice, refLastPrice)
				log.Printf("---------------------------------------------------------------")
				SaveReturnBackRecord(recordID, timeToReturn, maxDeviation, lastPriceChangeToRef, fairPriceChangeToRef)
				resetTickerState(symbol)
				return
			}

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

func SaveReturnBackRecord(recordID int64, returnTime time.Duration, maxDeviation float64, longProb float64, shortProb float64) {
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
