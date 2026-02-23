package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"habit-tracker-bot/internal/config"
	"habit-tracker-bot/internal/repository"
	"habit-tracker-bot/internal/server"
	"habit-tracker-bot/internal/service"
	"habit-tracker-bot/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	repo, err := repository.NewPostgresRepository(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer repo.Close()
	log.Println("Connected to database")

	bot, err := telegram.NewBot(cfg, repo)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	tinkoffSvc := service.NewTinkoffService(repo, cfg.TinkoffTerminalKey, cfg.TinkoffPassword, cfg.TinkoffTestMode)
	srv := server.NewServer(repo, tinkoffSvc, bot.GetHandlers(), cfg.Port)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
		cancel()
	}()

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	if err := bot.Start(ctx); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
