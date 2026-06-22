package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ─── Общее хранилище для всех бирж ───────────────────────────────────────────

var MultiState = &models.MultiExchangeState{
	Exchanges: make(map[string]*models.ExchangeState),
	Mu:        sync.Mutex{},
}

var multiClient = &http.Client{Timeout: 5 * time.Second}

// ─── Структура одного тикера с биржи ─────────────────────────────────────────

type RawTicker struct {
	Symbol    string
	LastPrice float64
	MarkPrice float64 // если есть (OKX, Hyperliquid)
	Volume24  float64
	Exchange  string
}

// toSplashData конвертирует в SplashData
// Биржи без markPrice: FairPrice = LastPrice (gap будет 0)
func (r RawTicker) toSplashData() models.SplashData {
	fair := r.MarkPrice
	if fair == 0 {
		fair = r.LastPrice
	}
	return models.SplashData{
		Symbol:    r.Symbol,
		LastPrice: r.LastPrice,
		FairPrice: fair,
		Volume24:  r.Volume24,
	}
}

// ─── Фетчеры ─────────────────────────────────────────────────────────────────

func fetchOKXTickers() ([]RawTicker, error) {
	resp, err := multiClient.Get("https://www.okx.com/api/v5/market/tickers?instType=SWAP")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			InstID  string `json:"instId"`
			Last    string `json:"last"`
			MarkPx  string `json:"markPx"`
			VolCcy  string `json:"volCcy24h"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []RawTicker
	for _, t := range raw.Data {
		if !strings.HasSuffix(t.InstID, "-USDT-SWAP") {
			continue
		}
		var last, mark, vol float64
		fmt.Sscanf(t.Last, "%f", &last)
		fmt.Sscanf(t.MarkPx, "%f", &mark)
		fmt.Sscanf(t.VolCcy, "%f", &vol)
		if last <= 0 {
			continue
		}
		sym := strings.TrimSuffix(t.InstID, "-USDT-SWAP") + "_USDT"
		result = append(result, RawTicker{Symbol: sym, LastPrice: last, MarkPrice: mark, Volume24: vol, Exchange: "OKX"})
	}
	return result, nil
}

func fetchGateTickers() ([]RawTicker, error) {
	resp, err := multiClient.Get("https://api.gateio.ws/api/v4/futures/usdt/tickers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw []struct {
		Contract  string  `json:"contract"`
		Last      string  `json:"last"`
		MarkPrice string  `json:"mark_price"`
		Vol24h    float64 `json:"volume_24h_quote"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []RawTicker
	for _, t := range raw {
		var last, mark float64
		fmt.Sscanf(t.Last, "%f", &last)
		fmt.Sscanf(t.MarkPrice, "%f", &mark)
		if last <= 0 {
			continue
		}
		result = append(result, RawTicker{Symbol: t.Contract, LastPrice: last, MarkPrice: mark, Volume24: t.Vol24h, Exchange: "Gate.io"})
	}
	return result, nil
}

func fetchBingXTickers() ([]RawTicker, error) {
	resp, err := multiClient.Get("https://open-api.bingx.com/openApi/swap/v2/quote/ticker")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			Symbol      string      `json:"symbol"`
			LastPrice   json.Number `json:"lastPrice"`
			MarkPrice   json.Number `json:"markPrice"`
			QuoteVolume json.Number `json:"quoteVolume"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []RawTicker
	for _, t := range raw.Data {
		var last, mark, vol float64
		fmt.Sscanf(t.LastPrice.String(), "%f", &last)
		fmt.Sscanf(t.MarkPrice.String(), "%f", &mark)
		fmt.Sscanf(t.QuoteVolume.String(), "%f", &vol)
		if last <= 0 {
			continue
		}
		// BTC-USDT → BTC_USDT
		sym := strings.ReplaceAll(t.Symbol, "-", "_")
		result = append(result, RawTicker{Symbol: sym, LastPrice: last, MarkPrice: mark, Volume24: vol, Exchange: "BingX"})
	}
	return result, nil
}

func fetchHyperliquidTickers() ([]RawTicker, error) {
	body, _ := json.Marshal(map[string]string{"type": "metaAndAssetCtxs"})
	resp, err := multiClient.Post("https://api.hyperliquid.xyz/info", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var raw [2]json.RawMessage
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, err
	}

	var meta struct {
		Universe []struct{ Name string `json:"name"` } `json:"universe"`
	}
	var ctxs []struct {
		MarkPx    string `json:"markPx"`
		MidPx     string `json:"midPx"`
		DayNtlVlm string `json:"dayNtlVlm"`
	}
	json.Unmarshal(raw[0], &meta)
	json.Unmarshal(raw[1], &ctxs)

	if len(meta.Universe) != len(ctxs) {
		return nil, fmt.Errorf("hyperliquid meta/ctx mismatch")
	}

	var result []RawTicker
	for i, asset := range meta.Universe {
		ctx := ctxs[i]
		px := ctx.MarkPx
		if px == "" {
			px = ctx.MidPx
		}
		var mark, vol float64
		fmt.Sscanf(px, "%f", &mark)
		fmt.Sscanf(ctx.DayNtlVlm, "%f", &vol)
		if mark <= 0 {
			continue
		}
		sym := strings.ToUpper(asset.Name) + "_USDT"
		result = append(result, RawTicker{Symbol: sym, LastPrice: mark, MarkPrice: mark, Volume24: vol, Exchange: "Hyperliquid"})
	}
	return result, nil
}

// ─── Главный поллер ───────────────────────────────────────────────────────────

type exchangeFetcher struct {
	name string
	fn   func() ([]RawTicker, error)
}

var exchangeFetchers = []exchangeFetcher{
	{"OKX", fetchOKXTickers},
	{"Gate.io", fetchGateTickers},
	{"BingX", fetchBingXTickers},
	{"Hyperliquid", fetchHyperliquidTickers},
}

func StartMultiExchangePolling(ctx context.Context) {
	log.Println("Multi-Exchange Splash Engine: Online")
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, ef := range exchangeFetchers {
				go pollExchange(ctx, ef)
			}
		}
	}
}

func pollExchange(ctx context.Context, ef exchangeFetcher) {
	tickers, err := ef.fn()
	if err != nil {
		log.Printf("Multi-splash [%s]: %v", ef.name, err)
		return
	}

	now := time.Now()

	MultiState.Mu.Lock()
	exState, exists := MultiState.Exchanges[ef.name]
	if !exists {
		exState = &models.ExchangeState{
			TickerStates: make(map[string]models.TickerState),
		}
		MultiState.Exchanges[ef.name] = exState
	}

	for _, t := range tickers {
		data := t.toSplashData()
		state, ok := exState.TickerStates[t.Symbol]

		if !ok {
			exState.TickerStates[t.Symbol] = models.TickerState{
				WindowStartRef:   data,
				LatestTickerData: data,
				LastRefUpdate:    now,
			}
			continue
		}

		if !state.SplashTrigger && now.Sub(state.LastRefUpdate) >= models.Window {
			state.WindowStartRef = data
			state.LastRefUpdate = now
			state.LastTriggeredLevel = 0
		}
		state.LatestTickerData = data
		exState.TickerStates[t.Symbol] = state
	}
	MultiState.Mu.Unlock()

	checkExchangePrices(ctx, ef.name, tickers, now)
}

func checkExchangePrices(ctx context.Context, exchange string, tickers []RawTicker, now time.Time) {
	for _, t := range tickers {
		MultiState.Mu.Lock()
		exState := MultiState.Exchanges[exchange]
		state, ok := exState.TickerStates[t.Symbol]
		if !ok {
			MultiState.Mu.Unlock()
			continue
		}

		ref := state.WindowStartRef
		if ref.LastPrice <= 0 || t.LastPrice <= 0 {
			MultiState.Mu.Unlock()
			continue
		}

		change := math.Abs(t.LastPrice-ref.LastPrice) / ref.LastPrice
		tier, triggered := GetNextSplash(change, state.LastTriggeredLevel)
		if !triggered {
			MultiState.Mu.Unlock()
			continue
		}
		if state.SplashTrigger && tier.Level <= state.LastTriggeredLevel*100 {
			MultiState.Mu.Unlock()
			continue
		}

		MultiState.Mu.Unlock()
		handleMultiSplash(ctx, exchange, t, tier, ref, now, state)
	}
}

func handleMultiSplash(ctx context.Context, exchange string, t RawTicker, tier models.SplashTier, ref models.SplashData, refTime time.Time, state models.TickerState) {
	now := time.Now()
	data := t.toSplashData()

	direction := "UP"
	if t.LastPrice < ref.LastPrice {
		direction = "DOWN"
	}

	targetLevel := int(math.Round(tier.Level))
	speed := now.Sub(refTime).Seconds()

	// Для бирж без markPrice gap = 0
	basisGap := 0.0
	if data.FairPrice > 0 && data.LastPrice != data.FairPrice {
		basisGap = math.Abs(data.LastPrice-data.FairPrice) / data.FairPrice * 100
	}

	prob := -1.0
	total, wins, err := database.GetContextStats(direction, targetLevel, t.Volume24, basisGap, tier.Window)
	if err == nil && total >= 3 {
		prob = math.Round(float64(wins) / float64(total) * 100)
	}

	if state.SplashTrigger {
		lastLevel := int(math.Round(state.LastTriggeredLevel * 100))
		if direction == state.SplashDirection && targetLevel > lastLevel {
			database.UpdateSplashLevel(state.SplashRecordID, targetLevel, t.LastPrice, data.FairPrice, t.Volume24, prob, tier.Window)

			MultiState.Mu.Lock()
			exState := MultiState.Exchanges[exchange]
			s := exState.TickerStates[t.Symbol]
			s.LastTriggeredLevel = float64(targetLevel) / 100.0
			s.CurrentTimeWindow = tier.Window
			exState.TickerStates[t.Symbol] = s
			MultiState.Mu.Unlock()

			emitMultiSplashEvent(ctx, exchange, t, data, ref, direction, tier, prob, basisGap, speed, "ACTIVE")
		}
		return
	}

	record := models.SplashRecord{
		Symbol: t.Symbol, Direction: direction, TriggerLevel: targetLevel,
		RefLastPrice: ref.LastPrice, RefFairPrice: ref.FairPrice,
		TriggerLastPrice: t.LastPrice, TriggerFairPrice: data.FairPrice,
		TriggerTime: now, Volume24h: t.Volume24, LongProbability: prob, TimeWindow: tier.Window,
	}
	recordID, err := database.SaveSplashRecord(record, basisGap, speed)
	if err != nil {
		return
	}

	MultiState.Mu.Lock()
	exState := MultiState.Exchanges[exchange]
	s := exState.TickerStates[t.Symbol]
	s.SplashRecordID = recordID
	s.LastTriggeredLevel = float64(targetLevel) / 100.0
	s.SplashTrigger = true
	s.TriggerTime = now
	s.SplashDirection = direction
	exState.TickerStates[t.Symbol] = s
	MultiState.Mu.Unlock()

	emitMultiSplashEvent(ctx, exchange, t, data, ref, direction, tier, prob, basisGap, speed, "ACTIVE")

	go TrackReturnBackMulti(recordID, exchange, t.Symbol, ref.LastPrice, ref.FairPrice, now, direction, tier.Window)
}

func emitMultiSplashEvent(ctx context.Context, exchange string, t RawTicker, data models.SplashData, ref models.SplashData, dir string, tier models.SplashTier, prob, gap, spd float64, status string) {
	if ctx == nil {
		return
	}
	runtime.EventsEmit(ctx, "splash:new", map[string]interface{}{
		"symbol":       t.Symbol,
		"exchange":     exchange,
		"direction":    dir,
		"level":        int(tier.Level),
		"activeWindow": tier.Window,
		"prob":         math.Round(prob),
		"refLast":      fmt.Sprintf("%.6f", ref.LastPrice),
		"refFair":      fmt.Sprintf("%.6f", ref.FairPrice),
		"lastPrice":    fmt.Sprintf("%.6f", t.LastPrice),
		"fairPrice":    fmt.Sprintf("%.6f", data.FairPrice),
		"gap":          fmt.Sprintf("%.2f", gap),
		"speed":        fmt.Sprintf("%.1f", spd),
		"volume":       t.Volume24,
		"timestamp":    time.Now().Format("15:04:05"),
		"status":       status,
	})
}

// TrackReturnBackMulti — аналог TrackReturnBack но для MultiState
func TrackReturnBackMulti(recordID int64, exchange, symbol string, refLast, refFair float64, triggerTime time.Time, direction string, windowMin int) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	warmup := 0
	maxDeviation := 0.0

	for range ticker.C {
		if warmup < 2 {
			warmup++
			continue
		}

		MultiState.Mu.Lock()
		exState, ok := MultiState.Exchanges[exchange]
		if !ok {
			MultiState.Mu.Unlock()
			return
		}
		state, ok := exState.TickerStates[symbol]
		MultiState.Mu.Unlock()

		if !ok || !state.SplashTrigger || state.SplashRecordID != recordID {
			return
		}

		currentWindow := state.CurrentTimeWindow
		if currentWindow == 0 {
			currentWindow = windowMin
		}

		tolerance := dynamicTolerance(state.LastTriggeredLevel)
		timeSince := time.Since(triggerTime)

		if timeSince > time.Duration(currentWindow)*time.Minute {
			SaveReturnBackRecord(recordID, false, timeSince, maxDeviation)
			resetMultiTickerState(exchange, symbol)
			return
		}

		cur := state.LatestTickerData
		if cur.LastPrice == 0 {
			continue
		}

		dev := math.Abs(cur.LastPrice-refLast) / refLast
		if dev > maxDeviation {
			maxDeviation = dev
		}

		if dev <= tolerance {
			SaveReturnBackRecord(recordID, true, time.Since(triggerTime), maxDeviation)
			resetMultiTickerState(exchange, symbol)
			return
		}
	}
}

func resetMultiTickerState(exchange, symbol string) {
	MultiState.Mu.Lock()
	defer MultiState.Mu.Unlock()
	exState, ok := MultiState.Exchanges[exchange]
	if !ok {
		return
	}
	state, ok := exState.TickerStates[symbol]
	if !ok {
		return
	}
	state.SplashTrigger = false
	state.TriggerTime = time.Time{}
	state.SplashDirection = ""
	state.LastTriggeredLevel = 0
	state.SplashRecordID = 0
	exState.TickerStates[symbol] = state
}
