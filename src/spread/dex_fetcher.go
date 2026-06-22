package spread

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"splash-trading-bot/lib/models"
	"strings"
	"time"
)

var dexClient = &http.Client{Timeout: 5 * time.Second}

// ─── Hyperliquid Perpetuals ───────────────────────────────────────────────────
// Hyperliquid — L1 DEX с нативными перп-фьючерсами.
// Публичный REST API без ключей: https://api.hyperliquid.xyz/info
//
// Метод allMids возвращает mid-price для всех торгуемых перпов.
// Метод metaAndAssetCtxs возвращает meta + funding + OI + объём за 24ч.

type hlInfoRequest struct {
	Type string `json:"type"`
}

type hlMetaResponse struct {
	Universe []struct {
		Name        string `json:"name"`
		MaxLeverage int    `json:"maxLeverage"`
	} `json:"universe"`
}

type hlAssetCtx struct {
	DayNtlVlm    string `json:"dayNtlVlm"` // объём за 24ч в USDC
	MarkPx       string `json:"markPx"`    // mark price
	MidPx        string `json:"midPx"`     // mid price (может быть null)
	OraclePx     string `json:"oraclePx"`  // oracle price
	OpenInterest string `json:"openInterest"`
}

// FetchHyperliquid возвращает цены всех перп-фьючерсов с Hyperliquid.
// Использует mark price как основную цену (аналог fairPrice на MEXC).
func FetchHyperliquid() ([]models.ExchangePrice, error) {
	// Один запрос metaAndAssetCtxs отдаёт и мету (список тикеров), и котировки
	reqBody, _ := json.Marshal(hlInfoRequest{Type: "metaAndAssetCtxs"})
	resp, err := dexClient.Post(
		"https://api.hyperliquid.xyz/info",
		"application/json",
		bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("hyperliquid: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Ответ — массив из двух элементов: [meta, []assetCtx]
	var raw [2]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("hyperliquid parse: %w", err)
	}

	var meta hlMetaResponse
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return nil, fmt.Errorf("hyperliquid meta: %w", err)
	}

	var ctxs []hlAssetCtx
	if err := json.Unmarshal(raw[1], &ctxs); err != nil {
		return nil, fmt.Errorf("hyperliquid ctxs: %w", err)
	}

	if len(meta.Universe) != len(ctxs) {
		return nil, fmt.Errorf("hyperliquid: meta/ctx length mismatch (%d vs %d)",
			len(meta.Universe), len(ctxs))
	}

	var result []models.ExchangePrice
	for i, asset := range meta.Universe {
		ctx := ctxs[i]

		// Приоритет цены: markPx → midPx → oraclePx
		priceStr := ctx.MarkPx
		if priceStr == "" || priceStr == "0" {
			priceStr = ctx.MidPx
		}
		if priceStr == "" || priceStr == "0" {
			priceStr = ctx.OraclePx
		}

		var price, vol float64
		fmt.Sscanf(priceStr, "%f", &price)
		fmt.Sscanf(ctx.DayNtlVlm, "%f", &vol)

		if price <= 0 {
			continue
		}

		// Hyperliquid тикеры уже чистые: "BTC", "ETH", "SOL"
		sym := strings.ToUpper(strings.TrimSpace(asset.Name))
		if sym == "" {
			continue
		}

		result = append(result, models.ExchangePrice{
			Exchange: "Hyperliquid",
			Price:    price,
			Volume24: vol,
			Source:   "DEX",
			Chain:    sym, // используем Chain для передачи нормализованного символа
			DexPair:  fmt.Sprintf("HL:%s", sym),
		})
	}
	return result, nil
}
