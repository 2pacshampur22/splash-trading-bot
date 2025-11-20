package internal

import (
	"encoding/json"
	"time"
)

const (
	MexcWsURl      = "wss://wbs-api.mexc.com/ws"
	FuturesRestAPI = "https://contract.mexc.com/api/v1/contract/ticker"

	SplashPercent = 0.01
	Window        = 10 * time.Second
)

type SubMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}

type WsMessage struct {
	Channel string          `json:"channel"`
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
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
