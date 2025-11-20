package processor

import (
	"encoding/json"
	"log"
	"splash-trading-bot/src/internal"
	"strconv"
	"strings"
	"sync"
	"time"
)

type SplashData struct {
	SplashData internal.SplashData
}

var (
	priceHistory = make(map[string][]internal.PriceRecord)
	historyMutex sync.Mutex
)

func (sd *SplashData) PriceToFloat() (float64, error) {
	return strconv.ParseFloat(sd.SplashData.Price, 64)
}

func WsMsgProcess(message []byte) {
	var wsMsg internal.WsMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		return
	}

	if wsMsg.Code != 0 || !strings.HasPrefix(wsMsg.Channel, "deal.") {
		return
	}

	var deals []SplashData

	if err := json.Unmarshal(wsMsg.Data, &deals); err != nil {
		log.Println("Cant decode deals data: %w", err)
		return
	}

	for _, deal := range deals {
		checkAndRecordSplash(deal)
	}
}

func checkAndRecordSplash(deal SplashData) {
	price, err := deal.PriceToFloat()
	if err != nil {
		log.Printf("Ошибка конвертации цены '%s': %v", deal.SplashData.Price, err)
		return
	}

	historyMutex.Lock()
	defer historyMutex.Unlock()

	symbol := deal.SplashData.Symbol
	newRecord := internal.PriceRecord{
		Price: price,
		Time:  deal.SplashData.Time,
	}
	currentHistory := priceHistory[symbol]
	currentHistory = append(currentHistory, newRecord)

	cutoffTime := deal.SplashData.Time - int64(internal.Window*1000)

	i := 0
	for i < len(currentHistory) && currentHistory[i].Time < cutoffTime {
		i++
	}
	currentHistory = currentHistory[i:]
	priceHistory[symbol] = currentHistory

	if len(currentHistory) < 2 {
		return
	}

	minPrice := currentHistory[0].Price
	maxPrice := currentHistory[0].Price

	for _, item := range currentHistory {
		if item.Price < minPrice {
			minPrice = item.Price
		}
		if item.Price > maxPrice {
			maxPrice = item.Price
		}
	}

	percentChange := (maxPrice - minPrice) / minPrice

	if percentChange >= internal.SplashPercent {
		log.Printf("	SPLASH ALERT [%s]", symbol)
		log.Printf("	DIFFERENCE: %.2f%% in ~%d seconds.",
			percentChange*100,
			internal.SplashPercent*100,
			internal.Window)
		log.Printf("   Диапазон: %.4f -> %.4f", minPrice, maxPrice)
		log.Printf("   Время события: %s", time.UnixMilli(deal.SplashData.Time).Format("15:04:05.000"))
	}
}
