package app

import (
	"context"
	"splash-trading-bot/lib/models"
	"splash-trading-bot/src/client"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

// startup вызывается при запуске приложения.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go client.StartPolling(a.ctx)
}

func (a *App) UpdateConfig(config models.EngineConfig) {
	models.CurrentConfig = config
	runtime.LogInfof(a.ctx, "Config successfully updated, %v levels updated", len(config.Tiers))
}
