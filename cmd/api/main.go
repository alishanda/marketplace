package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"marketplace/internal/app"
	"marketplace/internal/config"
)

func main() {
	app.InitLogger()
	cfg := config.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	a, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("startup", "err", err)
		os.Exit(1)
	}
	defer a.Close(context.Background())

	a.Start(ctx)
	slog.Info("listening", "url", a.URL())
	<-ctx.Done()

	shut, done := context.WithTimeout(context.Background(), 8*time.Second)
	defer done()
	a.Close(shut)
}
