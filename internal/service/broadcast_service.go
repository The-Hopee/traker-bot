package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

type BroadcastService struct {
	repo      repository.Repository
	bot       *tgbotapi.BotAPI
	mu        sync.Mutex
	isRunning bool
	stopChan  chan struct{}
}

func NewBroadcastService(repo repository.Repository, bot *tgbotapi.BotAPI) *BroadcastService {
	return &BroadcastService{
		repo:     repo,
		bot:      bot,
		stopChan: make(chan struct{}),
	}
}

func (s *BroadcastService) StartBroadcast(ctx context.Context, broadcastID int64) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("рассылка уже запущена")
	}
	s.isRunning = true
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	go s.runBroadcast(ctx, broadcastID)
	return nil
}

func (s *BroadcastService) StopBroadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		close(s.stopChan)
		s.isRunning = false
	}
}

func (s *BroadcastService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

func (s *BroadcastService) runBroadcast(ctx context.Context, broadcastID int64) {
	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	broadcast, err := s.repo.GetBroadcastByID(ctx, broadcastID)
	if err != nil {
		log.Printf("Broadcast error: %v", err)
		return
	}

	totalUsers, _ := s.repo.GetTotalUsersCount(ctx)
	s.repo.StartBroadcast(ctx, broadcastID, totalUsers)

	lastUserID := broadcast.LastUserID
	sentCount := broadcast.SentCount
	failedCount := broadcast.FailedCount
	batchSize := 25

	for {
		select {
		case <-s.stopChan:
			s.repo.UpdateBroadcastStatus(ctx, broadcastID, domain.BroadcastPaused)
			log.Printf("Broadcast %d paused at user %d", broadcastID, lastUserID)
			return
		default:
		}

		userIDs, maxID, err := s.repo.GetUsersForBroadcast(ctx, lastUserID, batchSize)
		if err != nil {
			log.Printf("Error getting users: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if len(userIDs) == 0 {
			s.repo.CompleteBroadcast(ctx, broadcastID)
			log.Printf("Broadcast %d completed: sent=%d, failed=%d", broadcastID, sentCount, failedCount)
			return
		}

		for _, telegramID := range userIDs {
			err := s.sendBroadcastMessage(telegramID, broadcast)
			if err != nil {
				failedCount++
				log.Printf("Failed to send to %d: %v", telegramID, err)
			} else {
				sentCount++
			}
			time.Sleep(40 * time.Millisecond)
		}

		lastUserID = maxID
		s.repo.UpdateBroadcastProgress(ctx, broadcastID, sentCount, failedCount, lastUserID)
	}
}

func (s *BroadcastService) sendBroadcastMessage(telegramID int64, broadcast *domain.Broadcast) error {
	var msg tgbotapi.Chattable

	if broadcast.ImageURL != nil && *broadcast.ImageURL != "" {
		photo := tgbotapi.NewPhoto(telegramID, tgbotapi.FileURL(*broadcast.ImageURL))
		photo.Caption = broadcast.Text
		photo.ParseMode = "Markdown"
		if broadcast.ButtonText != nil && broadcast.ButtonURL != nil && *broadcast.ButtonText != "" && *broadcast.ButtonURL != "" {
			photo.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL(*broadcast.ButtonText, *broadcast.ButtonURL),
				),
			)
		}
		msg = photo
	} else {
		textMsg := tgbotapi.NewMessage(telegramID, broadcast.Text)
		textMsg.ParseMode = "Markdown"
		if broadcast.ButtonText != nil && broadcast.ButtonURL != nil && *broadcast.ButtonText != "" && *broadcast.ButtonURL != "" {
			textMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL(*broadcast.ButtonText, *broadcast.ButtonURL),
				),
			)
		}
		msg = textMsg
	}

	_, err := s.bot.Send(msg)
	return err
}
func (s *BroadcastService) ResumeBroadcast(ctx context.Context) error {
	broadcast, err := s.repo.GetRunningBroadcast(ctx)
	if err != nil {
		broadcasts, _ := s.repo.GetAllBroadcasts(ctx)
		for _, b := range broadcasts {
			if b.Status == domain.BroadcastPaused {
				broadcast = b
				break
			}
		}
	}

	if broadcast == nil {
		return fmt.Errorf("нет рассылки для продолжения")
	}

	return s.StartBroadcast(ctx, broadcast.ID)
}
