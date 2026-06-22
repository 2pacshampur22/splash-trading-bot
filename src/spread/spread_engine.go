package spread

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"splash-trading-bot/database"
	"splash-trading-bot/lib/models"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	latestPrices   = make(map[string][]models.ExchangePrice)
	latestPricesMu sync.RWMutex
)

// StartSpreadPolling — запускается из app.go в отдельной горутине
func StartSpreadPolling(ctx context.Context) {
	log.Println("Spread Engine: Online")
	for {
		cfg := models.CurrentSpreadConfig
		interval := time.Duration(cfg.PollingIntervalMs) * time.Millisecond
		if interval < 500*time.Millisecond {
			interval = 500 * time.Millisecond
		}
		select {
		case <-ctx.Done():
			log.Println("Spread Engine: Stopped")
			return
		case <-time.After(interval):
		}
		runSpreadCycle(ctx)
	}
}

func runSpreadCycle(ctx context.Context) {
	cfg := models.CurrentSpreadConfig

	// Шаг 1: все CEX фьючерсные биржи параллельно
	merged := make(map[string][]models.ExchangePrice)

	if cfg.EnableCEX {
		cexMap := fetchCEXFuturesParallel()
		for sym, prices := range cexMap {
			merged[sym] = append(merged[sym], prices...)
		}
	}

	// Шаг 2: Hyperliquid
	if cfg.EnableDEX {
		hlPrices, err := FetchHyperliquid()
		if err != nil {
			log.Printf("Hyperliquid fetch error: %v", err)
		} else {
			for _, ep := range hlPrices {
				sym := ep.Chain // Chain = нормализованный символ
				ep.Chain = ""   // очищаем, Chain не нужен для перпов
				merged[sym] = append(merged[sym], ep)
			}
		}
	}

	latestPricesMu.Lock()
	latestPrices = merged
	latestPricesMu.Unlock()

	// Шаг 3: ищем спреды и шлём в UI
	signals := detectSpreads(merged, cfg)
	for _, sig := range signals {
		runtime.EventsEmit(ctx, "spread:new", sig)
		if sig.IsAlert {
			record := models.SpreadRecord{
				Symbol:       sig.Symbol,
				BuyExchange:  sig.BuyExchange,
				SellExchange: sig.SellExchange,
				BuyPrice:     sig.BuyPrice,
				SellPrice:    sig.SellPrice,
				SpreadPct:    sig.SpreadPct,
				Volume24h:    sig.Volume24h,
				Source:       sig.Source,
				DetectedAt:   time.Now(),
			}
			if err := database.SaveSpreadRecord(record); err != nil {
				log.Printf("Spread DB error: %v", err)
			}
		}
	}
}

// fetchCEXFuturesParallel запускает MEXC, OKX, Gate.io, BingX одновременно
// и возвращает map[normalizedSymbol][]ExchangePrice
func fetchCEXFuturesParallel() map[string][]models.ExchangePrice {
	type namedResult struct {
		name   string
		prices []models.ExchangePrice
		err    error
	}

	fetchers := []struct {
		name string
		fn   func() ([]models.ExchangePrice, error)
	}{
		{"MEXC", FetchMEXCFutures},
		{"OKX", FetchOKXFutures},
		{"Gate.io", FetchGateFutures},
		{"BingX", FetchBingXFutures},
	}

	ch := make(chan namedResult, len(fetchers))
	for _, f := range fetchers {
		go func(name string, fn func() ([]models.ExchangePrice, error)) {
			prices, err := fn()
			ch <- namedResult{name, prices, err}
		}(f.name, f.fn)
	}

	result := make(map[string][]models.ExchangePrice)
	for range fetchers {
		r := <-ch
		if r.err != nil {
			log.Printf("CEX futures [%s]: %v", r.name, r.err)
			continue
		}
		for _, ep := range r.prices {
			sym := ep.Chain // Chain хранит нормализованный символ внутри фетчеров
			ep.Chain = ""   // не нужен снаружи
			if sym != "" {
				result[sym] = append(result[sym], ep)
			}
		}
	}
	return result
}

// detectSpreads находит пару (min, max) цены для каждого символа
func detectSpreads(priceMap map[string][]models.ExchangePrice, cfg models.SpreadConfig) []models.SpreadSignal {
	var signals []models.SpreadSignal
	now := time.Now().Format("15:04:05")

	// Минимальный объём на каждой отдельной бирже — делистнутый токен
	// на Hyperliquid имеет объём 0 или < $1000 при живом рынке на CEX
	const minPerExchangeVol = 1000.0

	for sym, prices := range priceMap {
		if len(prices) < 2 {
			continue
		}

		// Фильтруем цены с нулевым или мизерным объёмом на конкретной бирже —
		// замерший делистнутый токен будет иметь dayNtlVlm = 0
		var active []models.ExchangePrice
		for _, p := range prices {
			if p.Volume24 >= minPerExchangeVol {
				active = append(active, p)
			}
		}
		if len(active) < 2 {
			continue
		}

		// Фильтр по суммарному объёму (макс среди активных бирж)
		maxVol := 0.0
		for _, p := range active {
			if p.Volume24 > maxVol {
				maxVol = p.Volume24
			}
		}
		if maxVol < cfg.MinVolume24h {
			continue
		}

		minP, maxP := active[0], active[0]
		for _, p := range active[1:] {
			if p.Price < minP.Price {
				minP = p
			}
			if p.Price > maxP.Price {
				maxP = p
			}
		}

		if minP.Price <= 0 {
			continue
		}

		spreadPct := ((maxP.Price - minP.Price) / minP.Price) * 100
		spreadPct = math.Round(spreadPct*1000) / 1000

		if spreadPct < 0.05 {
			continue
		}

		// Фильтр ложных совпадений: разные токены с одним тикером на разных биржах
		if maxP.Price/minP.Price > 10 {
			continue
		}

		source := classifySource(minP.Source, maxP.Source)
		isAlert := spreadPct >= cfg.AlertThresholdPct

		signals = append(signals, models.SpreadSignal{
			Symbol:       sym,
			BuyExchange:  minP.Exchange,
			SellExchange: maxP.Exchange,
			BuyPrice:     minP.Price,
			SellPrice:    maxP.Price,
			SpreadPct:    spreadPct,
			Volume24h:    maxVol,
			Source:       source,
			Chain:        "", // у перпов нет чейна
			Timestamp:    now,
			IsAlert:      isAlert,
		})
	}

	sort.Slice(signals, func(i, j int) bool {
		return signals[i].SpreadPct > signals[j].SpreadPct
	})

	if len(signals) > 100 {
		signals = signals[:100]
	}
	return signals
}

func classifySource(a, b string) string {
	if a == "DEX" && b == "DEX" {
		return "DEX"
	}
	if a == "CEX" && b == "CEX" {
		return "CEX"
	}
	return "CEX-DEX"
}

func UpdateSpreadConfig(cfg models.SpreadConfig) {
	models.CurrentSpreadConfig = cfg
	log.Printf("Spread config updated: threshold=%.2f%% minVol=%.0f cex=%v dex=%v",
		cfg.AlertThresholdPct, cfg.MinVolume24h, cfg.EnableCEX, cfg.EnableDEX)
}

func GetLatestSpreads() []models.SpreadSignal {
	latestPricesMu.RLock()
	defer latestPricesMu.RUnlock()
	return detectSpreads(latestPrices, models.CurrentSpreadConfig)
}

// Заглушка — использовалась в старом cex_fetcher.go, убрана
var _ = fmt.Sprintf
