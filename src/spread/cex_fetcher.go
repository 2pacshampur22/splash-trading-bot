package spread

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"splash-trading-bot/lib/models"
	"strings"
	"time"
)

var cexClient = &http.Client{Timeout: 8 * time.Second}

// ─── MEXC Futures ─────────────────────────────────────────────────────────────
// API: https://contract.mexc.com/api/v1/contract/ticker
// Уже используется в основном парсере — переиспользуем тот же эндпоинт

func FetchMEXCFutures() ([]models.ExchangePrice, error) {
	resp, err := cexClient.Get("https://contract.mexc.com/api/v1/contract/ticker")
	if err != nil {
		return nil, fmt.Errorf("mexc futures: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			Symbol    string  `json:"symbol"`    // BTC_USDT
			LastPrice float64 `json:"lastPrice"`
			Amount24  float64 `json:"amount24"`  // объём в USDT
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []models.ExchangePrice
	for _, t := range raw.Data {
		if t.LastPrice <= 0 {
			continue
		}
		// BTC_USDT → BTC
		base := stripFuturesSuffix(t.Symbol)
		if base == "" {
			continue
		}
		result = append(result, models.ExchangePrice{
			Exchange: "MEXC",
			Price:    t.LastPrice,
			Volume24: t.Amount24,
			Source:   "CEX",
			Chain:    base, // временно храним нормализованный символ
		})
	}
	return result, nil
}

// ─── OKX Futures (SWAP — вечные перпы) ───────────────────────────────────────
// API: https://www.okx.com/api/v5/market/tickers?instType=SWAP

func FetchOKXFutures() ([]models.ExchangePrice, error) {
	resp, err := cexClient.Get("https://www.okx.com/api/v5/market/tickers?instType=SWAP")
	if err != nil {
		return nil, fmt.Errorf("okx futures: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			InstID    string `json:"instId"`    // BTC-USDT-SWAP
			Last      string `json:"last"`
			VolCcy24h string `json:"volCcy24h"` // объём в базовой валюте
			Vol24h    string `json:"vol24h"`    // объём в контрактах
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []models.ExchangePrice
	for _, t := range raw.Data {
		// берём только USDT-маржинальные перпы
		if !strings.HasSuffix(t.InstID, "-USDT-SWAP") {
			continue
		}
		var price, vol float64
		fmt.Sscanf(t.Last, "%f", &price)
		fmt.Sscanf(t.VolCcy24h, "%f", &vol)
		if price <= 0 {
			continue
		}
		// BTC-USDT-SWAP → BTC
		base := strings.TrimSuffix(t.InstID, "-USDT-SWAP")
		result = append(result, models.ExchangePrice{
			Exchange: "OKX",
			Price:    price,
			Volume24: vol,
			Source:   "CEX",
			Chain:    base,
		})
	}
	return result, nil
}

// ─── Gate.io Futures (вечные контракты) ──────────────────────────────────────
// API: https://api.gateio.ws/api/v4/futures/usdt/tickers

func FetchGateFutures() ([]models.ExchangePrice, error) {
	resp, err := cexClient.Get("https://api.gateio.ws/api/v4/futures/usdt/tickers")
	if err != nil {
		return nil, fmt.Errorf("gate futures: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw []struct {
		Contract    string  `json:"contract"`     // BTC_USDT
		Last        string  `json:"last"`
		Volume24h   float64 `json:"volume_24h_quote"` // объём в USDT
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []models.ExchangePrice
	for _, t := range raw {
		var price float64
		fmt.Sscanf(t.Last, "%f", &price)
		if price <= 0 {
			continue
		}
		base := stripFuturesSuffix(t.Contract)
		if base == "" {
			continue
		}
		result = append(result, models.ExchangePrice{
			Exchange: "Gate.io",
			Price:    price,
			Volume24: t.Volume24h,
			Source:   "CEX",
			Chain:    base,
		})
	}
	return result, nil
}

// ─── BingX Futures (перп своп) ────────────────────────────────────────────────
// API: https://open-api.bingx.com/openApi/swap/v2/quote/ticker

func FetchBingXFutures() ([]models.ExchangePrice, error) {
	resp, err := cexClient.Get("https://open-api.bingx.com/openApi/swap/v2/quote/ticker")
	if err != nil {
		return nil, fmt.Errorf("bingx futures: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw struct {
		Data []struct {
			Symbol      string      `json:"symbol"`
			LastPrice   json.Number `json:"lastPrice"`
			Volume      json.Number `json:"volume"`
			QuoteVolume json.Number `json:"quoteVolume"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	var result []models.ExchangePrice
	for _, t := range raw.Data {
		var price, vol float64
		fmt.Sscanf(t.LastPrice.String(), "%f", &price)
		fmt.Sscanf(t.QuoteVolume.String(), "%f", &vol)
		if price <= 0 {
			continue
		}
		// BTC-USDT → BTC
		base := stripFuturesSuffix(t.Symbol)
		if base == "" {
			continue
		}
		result = append(result, models.ExchangePrice{
			Exchange: "BingX",
			Price:    price,
			Volume24: vol,
			Source:   "CEX",
			Chain:    base,
		})
	}
	return result, nil
}

// stripFuturesSuffix нормализует тикер фьючерса в базовый символ.
// BTC_USDT → BTC, BTC-USDT → BTC, BTCUSDT → BTC, ETH_USD → ETH
func stripFuturesSuffix(raw string) string {
	s := strings.ToUpper(raw)
	// убираем разделители
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	// убираем котировочные суффиксы
	for _, q := range []string{"USDTPERP", "USDT", "USDC", "USD", "BUSD"} {
		if strings.HasSuffix(s, q) {
			base := strings.TrimSuffix(s, q)
			if len(base) >= 2 {
				return base
			}
		}
	}
	return ""
}
