package client

import (
	"log"
	"math"
	"splash-trading-bot/lib/models"
	"time"
)

func TrackReturnBack(
	symbol string,
	refLastPrice float64,
	refFairPrice float64,
	triggerTime time.Time,
	direction string,
) {
	ticker := time.NewTicker(50 * time.Millisecond)

	for range ticker.C {

		TockenState.Mu.Lock()
		state, ok := TockenState.TickerStates[symbol]
		TockenState.Mu.Unlock()

		if !ok || !state.SplashTrigger {
			return
		}

		currentData := state.LatestTickerData
		lastPriceChangeToRef := math.Abs(currentData.LastPrice-refLastPrice) / refLastPrice
		fairPriceChangeToRef := math.Abs(currentData.FairPrice-refFairPrice) / refFairPrice

		if lastPriceChangeToRef <= models.ReturnTolerance && fairPriceChangeToRef <= models.ReturnTolerance {
			timeToReturn := time.Since(triggerTime)
			TockenState.Mu.Lock()

			state.SplashTrigger = false
			state.TriggerTime = time.Time{}
			state.SplashDirection = ""

			TockenState.TickerStates[symbol] = state
			TockenState.Mu.Unlock()
			if timeToReturn.Minutes() >= 1 {
				log.Printf("---------------------------------------------------------------")
				log.Printf("PRICE RETURNED: %s | LEVEL: %.0f%% | RETURN BACK TIME: %.2f min",
					symbol, state.LastTriggeredLevel*100, timeToReturn.Minutes())
				log.Printf("---------------------------------------------------------------")
				return
			} else {
				log.Printf("---------------------------------------------------------------")
				log.Printf("PRICE RETURNED: %s | LEVEL: %.0f%% | RETURN BACK TIME: %.2f sec",
					symbol, state.LastTriggeredLevel*100, timeToReturn.Seconds())
				log.Printf("---------------------------------------------------------------")
				return
			}

		}

	}
}
