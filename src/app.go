package app

import (
	"context"
	"splash-trading-bot/src/client"
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
	go client.StartPolling()
}
