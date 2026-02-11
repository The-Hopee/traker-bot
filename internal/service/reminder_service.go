package service

import (
	"context"
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"habit-tracker-bot/internal/repository"
)

type ReminderService struct {
	repo   repository.Repository
	cron   *cron.Cron
	notify func(telegramID int64, habitName string) error
}

func NewReminderService(repo repository.Repository) *ReminderService {
	return &ReminderService{
		repo: repo,
		cron: cron.New(),
	}
}

func (s *ReminderService) SetNotifyFunc(fn func(telegramID int64, habitName string) error) {
	s.notify = fn
}

func (s *ReminderService) Start() {
	s.cron.AddFunc("* * * * *", func() {
		s.checkReminders()
	})
	s.cron.Start()
	log.Println("Reminder service started")
}

func (s *ReminderService) Stop() {
	s.cron.Stop()
}

func (s *ReminderService) checkReminders() {
	ctx := context.Background()
	currentTime := time.Now().Format("15:04")

	habits, err := s.repo.GetHabitsForReminder(ctx, currentTime)
	if err != nil {
		log.Printf("Error getting habits for reminder: %v", err)
		return
	}

	for _, habit := range habits {
		telegramID, err := s.repo.GetUserTelegramIDByHabitID(ctx, habit.ID)
		if err != nil {
			log.Printf("Error getting user telegram ID: %v", err)
			continue
		}

		if s.notify != nil {
			if err := s.notify(telegramID, habit.Name); err != nil {
				log.Printf("Error sending reminder: %v", err)
			}
		}
	}
}
