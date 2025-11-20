package internal

import (
	"encoding/json"
	"time"
)

const (
	MexcWsURl      = "wss://wbs-api.mexc.com/ws"
	FuturesRestAPI = "https://api.mexc.com/api/v3/defaultSymbols"

	SplashPercent = 0.005
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
	Code    int      `json:"code"`
	Msg     string   `json:"msg"`
	Data    []string `json:"data"`
	Success bool     `json:"success"`
}
type PriceRecord struct {
	Price float64
	Time  int64
}

type SplashData struct {
	Symbol string `json:"symbol"`
	Price  string `json:"p"`
	Volume string `json:"v"`
	Time   int64  `json:"t"`
	Side   string `json:"s"`
}
