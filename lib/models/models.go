package models

import (
	"sync"
	"time"
)

const (
	FuturesRestAPI  = "https://contract.mexc.com/api/v1/contract/ticker"
	Window          = 5 * time.Minute
	ReturnTolerance = 0.005
)

var SplashLevels = []float64{
	0.01,
	0.03,
	0.05,
	0.12,
	0.24,
	0.48,
	0.96,
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
	Volume24  int64   `json:"volume24"`
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

	UpdateChan chan SplashData
}

type SharedState struct {
	TickerStates map[string]TickerState
	Mu           sync.Mutex
}
