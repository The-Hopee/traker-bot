package telegram

import (
	"context"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"habit-tracker-bot/internal/config"
	"habit-tracker-bot/internal/repository"
	"habit-tracker-bot/internal/service"
)

type Bot struct {
	api          *tgbotapi.BotAPI
	handlers     *Handlers
	adminHandler *AdminHandlers
	reminderSvc  *service.ReminderService
	broadcastSvc *service.BroadcastService
	cfg          *config.Config
}

func NewBot(cfg *config.Config, repo repository.Repository) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}

	api.Debug = cfg.Environment == "development"
	log.Printf("Authorized on account %s", api.Self.UserName)

	botUsername := cfg.BotUsername
	if botUsername == "" {
		botUsername = api.Self.UserName
	}

	// Services
	habitSvc := service.NewHabitService(repo)
	subSvc := service.NewSubscriptionService(repo, cfg.SubscriptionPrice)
	referralSvc := service.NewReferralService(repo, subSvc)
	achievementSvc := service.NewAchievementService(repo, subSvc)
	tinkoffSvc := service.NewTinkoffService(repo, cfg.TinkoffTerminalKey, cfg.TinkoffPassword, cfg.TinkoffTestMode)
	adSvc := service.NewAdService(repo)
	exportSvc := service.NewExportService(repo)
	reminderSvc := service.NewReminderService(repo)
	broadcastSvc := service.NewBroadcastService(repo, api)

	// Handlers
	handlers := NewHandlers(api, repo, habitSvc, subSvc, referralSvc, achievementSvc, tinkoffSvc, adSvc, exportSvc, botUsername, cfg.SubscriptionPrice)
	adminHandlers := NewAdminHandlers(api, repo, broadcastSvc, adSvc)
	handlers.SetAdminHandlers(adminHandlers)

	reminderSvc.SetNotifyFunc(handlers.SendReminder)

	// Add admin
	if cfg.AdminTelegramID != 0 {
		repo.AddAdmin(context.Background(), cfg.AdminTelegramID)
	}

	return &Bot{
		api:          api,
		handlers:     handlers,
		adminHandler: adminHandlers,
		reminderSvc:  reminderSvc,
		broadcastSvc: broadcastSvc,
		cfg:          cfg,
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.reminderSvc.Start()
	defer b.reminderSvc.Stop()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := b.api.GetUpdatesChan(updateConfig)

	log.Println("Bot started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Bot stopped")
			return nil
		case update := <-updates:
			go b.handlers.HandleUpdate(update)
		}
	}
}

func (b *Bot) GetHandlers() *Handlers {
	return b.handlers
}

func (b *Bot) GetBroadcastService() *service.BroadcastService {
	return b.broadcastSvc
}
