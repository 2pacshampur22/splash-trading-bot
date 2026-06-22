package models

import (
	"context"
	"sync"
	"time"
)

const (
	FuturesRestAPI  = "https://contract.mexc.com/api/v1/contract/ticker"
	Window          = 5 * time.Minute
	ReturnTolerance = 0.005
)

var AppCtx context.Context

// ─── Splash types ─────────────────────────────────────────────────────────────

type SplashTier struct {
	Level       float64 `json:"level"`
	Window      int     `json:"window"`
	IsForcedPin bool    `json:"isForcedPin"`
}

type EngineConfig struct {
	Tiers []SplashTier `json:"tiers"`
}

type Responce struct {
	Code    int          `json:"code"`
	Msg     string       `json:"msg"`
	Data    []SplashData `json:"data"`
	Success bool         `json:"success"`
}

type PriceRecord struct {
	Price float64
	Time  int64
}

type SplashData struct {
	Symbol    string  `json:"symbol"`
	LastPrice float64 `json:"lastPrice"`
	FairPrice float64 `json:"fairPrice"`
	Volume24  float64 `json:"amount24"`
}

type SplashRecord struct {
	ID               int
	Symbol           string
	Direction        string
	TriggerLevel     int
	TimeWindow       int
	RefLastPrice     float64
	RefFairPrice     float64
	TriggerLastPrice float64
	TriggerFairPrice float64
	TriggerTime      time.Time
	Volume24h        float64
	Returned         bool
	ReturnTime       time.Duration
	MaxDeviation     float64
	LongProbability  float64
	ShortProbability float64
}

type TickerState struct {
	WindowStartRef     SplashData
	LatestTickerData   SplashData
	LastTriggeredLevel float64
	SplashTrigger      bool
	TriggerTime        time.Time
	LastRefUpdate      time.Time
	CurrentTimeWindow  int
	SplashDirection    string
	SplashRecordID     int64

	UpdateChan chan SplashData
}

type SharedState struct {
	TickerStates map[string]TickerState
	Mu           sync.Mutex
}

// MultiExchangeState представляет общее потокобезопасное хранилище состояния для всех бирж
type MultiExchangeState struct {
	Exchanges map[string]*ExchangeState
	Mu        sync.Mutex
}

// ExchangeState хранит состояния тикеров для конкретной биржи
type ExchangeState struct {
	TickerStates map[string]TickerState
}

var CurrentConfig = EngineConfig{
	Tiers: []SplashTier{
		{Level: 3, Window: 10, IsForcedPin: false},
		{Level: 5, Window: 15, IsForcedPin: false},
	},
}

// ─── Spread types ─────────────────────────────────────────────────────────────

type SpreadRecord struct {
	ID           int
	Symbol       string
	BuyExchange  string
	SellExchange string
	BuyPrice     float64
	SellPrice    float64
	SpreadPct    float64
	Volume24h    float64
	Source       string // "CEX" | "DEX" | "CEX-DEX"
	DetectedAt   time.Time
}

type ExchangePrice struct {
	Exchange string
	Price    float64
	Volume24 float64
	Source   string // "CEX" | "DEX"
	Chain    string // внутренний: нормализованный символ внутри фетчеров
	DexPair  string // для DEX: идентификатор пары
}

type SpreadSignal struct {
	Symbol       string  `json:"symbol"`
	BuyExchange  string  `json:"buyExchange"`
	SellExchange string  `json:"sellExchange"`
	BuyPrice     float64 `json:"buyPrice"`
	SellPrice    float64 `json:"sellPrice"`
	SpreadPct    float64 `json:"spreadPct"`
	Volume24h    float64 `json:"volume24h"`
	Source       string  `json:"source"`
	Chain        string  `json:"chain"`
	Timestamp    string  `json:"timestamp"`
	IsAlert      bool    `json:"isAlert"`
}

type SpreadConfig struct {
	AlertThresholdPct float64 `json:"alertThresholdPct"`
	MinVolume24h      float64 `json:"minVolume24h"`
	EnableCEX         bool    `json:"enableCex"`
	EnableDEX         bool    `json:"enableDex"`
	PollingIntervalMs int     `json:"pollingIntervalMs"`
}

var CurrentSpreadConfig = SpreadConfig{
	AlertThresholdPct: 1.0,
	MinVolume24h:      500000,
	EnableCEX:         true,
	EnableDEX:         true,
	PollingIntervalMs: 2000,
}
