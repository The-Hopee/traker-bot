package service

import (
	"context"
	"log"
	"time"

	"habit-tracker-bot/internal/repository"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type PromoService struct {
	bot  *tgbotapi.BotAPI
	repo repository.Repository
}

func NewPromoService(bot *tgbotapi.BotAPI, repo repository.Repository) *PromoService {
	return &PromoService{
		bot:  bot,
		repo: repo,
	}
}

func (s *PromoService) Start(ctx context.Context) {
	// Проверяем каждый час
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	log.Println("Promo service started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAndSendPromos(ctx)
		}
	}
}

func (s *PromoService) checkAndSendPromos(ctx context.Context) {
	moscowLocation, _ := time.LoadLocation("Europe/Moscow")
	now := time.Now().In(moscowLocation)

	// Отправляем первую рекламу юзерам, которые с нами >= 1 день
	s.sendFirstPromo(ctx)

	// Еженедельная реклама — понедельник в 10:00
	if now.Weekday() == time.Monday && now.Hour() == 10 {
		s.sendWeeklyPromo(ctx)
	}
}

func (s *PromoService) sendFirstPromo(ctx context.Context) {
	users, err := s.repo.GetUsersForFirstPromo(ctx)
	if err != nil {
		log.Printf("Error getting users for first promo: %v", err)
		return
	}

	for _, userID := range users {
		text := `👋 Привет! Ты уже день с нами!

Попробуй другие наши полезные боты:

🎯 @BotName1 — описание бота
📝 @BotName2 — описание бота
💰 @BotName3 — описание бота

Каждый из них поможет стать продуктивнее! 🚀`

		keyboard := tgbotapi.NewInlineKeyboardMarkup( // сюда добавлять список ботов для рекламы
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("🎯 BotName1", "https://t.me/BotName1"),
				tgbotapi.NewInlineKeyboardButtonURL("📝 BotName2", "https://t.me/BotName2"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonURL("💰 BotName3", "https://t.me/BotName3"),
			),
		)

		msg := tgbotapi.NewMessage(userID, text)
		msg.ReplyMarkup = keyboard

		_, err := s.bot.Send(msg)
		if err != nil {
			log.Printf("Error sending first promo to %d: %v", userID, err)
			continue
		}

		s.repo.MarkFirstPromoSent(ctx, userID)

		// Пауза чтобы не словить лимит
		time.Sleep(50 * time.Millisecond)
	}
}

func (s *PromoService) sendWeeklyPromo(ctx context.Context) {
	users, err := s.repo.GetUsersForWeeklyPromo(ctx)
	if err != nil {
		log.Printf("Error getting users for weekly promo: %v", err)
		return
	}

	for _, userID := range users {
		text := `🌟 Новая неделя — новые возможности!

Не забудь про свои привычки 💪

А ещё загляни к друзьям:
🎯 @BotName1 — описание
📝 @BotName2 — описание`

		msg := tgbotapi.NewMessage(userID, text)

		_, err := s.bot.Send(msg)
		if err != nil {
			continue
		}

		s.repo.MarkWeeklyPromoSent(ctx, userID)
		time.Sleep(50 * time.Millisecond)
	}
}
