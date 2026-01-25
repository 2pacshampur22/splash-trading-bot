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
	Success bool         `json: "success"`
}
type PriceRecord struct {
	Price float64
	Time  int64
}

type SplashData struct {
	Symbol    string  `json:"symbol"`
	LastPrice float64 `json:"lastPrice"`
	FairPrice float64 `json:"fairPrice"`
	Volume24  int64   `json:"volume24"` //TODO: Заменить на amount24 - usdt volume
}

type SplashRecord struct {
	ID               int
	Symbol           string
	Direction        string
	TriggerLevel     int
	RefLastPrice     float64
	RefFairPrice     float64
	TriggerLastPrice float64
	TriggerFairPrice float64
	TriggerTime      time.Time
	Volume24h        int64
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
	SplashDirection    string
	SplashRecordID     int64

	UpdateChan chan SplashData
}

type SharedState struct {
	TickerStates map[string]TickerState
	Mu           sync.Mutex
}

var CurrentConfig = EngineConfig{
	Tiers: []SplashTier{
		{Level: 3, Window: 10, IsForcedPin: false},
		{Level: 5, Window: 15, IsForcedPin: false},
	},
}
