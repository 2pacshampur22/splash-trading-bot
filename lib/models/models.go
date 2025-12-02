package models

import (
	"time"
)

const (
	FuturesRestAPI = "https://contract.mexc.com/api/v1/contract/ticker"
	Window         = 5 * time.Minute
)

var SplashLevels = []float64{
	0.01,
	0.03,
	0.06,
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

type TickerState struct {
	Reference          SplashData
	LastTriggeredLevel float64
}
