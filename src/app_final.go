package app

import (
	"context"
	"splash-trading-bot/lib/models"
	"splash-trading-bot/src/client"
	"splash-trading-bot/src/spread"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go client.StartPolling(a.ctx)           // MEXC сплеш (оригинальный)
	go client.StartMultiExchangePolling(a.ctx) // OKX, Gate.io, BingX, Hyperliquid
	go spread.StartSpreadPolling(a.ctx)     // спред движок
}

func (a *App) UpdateConfig(config models.EngineConfig) {
	models.CurrentConfig = config
	runtime.LogInfof(a.ctx, "Splash config updated: %d tiers", len(config.Tiers))
}

func (a *App) UpdateSpreadConfig(config models.SpreadConfig) {
	spread.UpdateSpreadConfig(config)
}

func (a *App) GetLatestSpreads() []models.SpreadSignal {
	return spread.GetLatestSpreads()
}
