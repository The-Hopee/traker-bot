package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

var (
	ErrHabitLimitReached = errors.New("достигнут лимит привычек")
	ErrHabitNotFound     = errors.New("привычка не найдена")
	ErrAccessDenied      = errors.New("доступ запрещён")
)

type HabitService struct {
	repo repository.Repository
}

func NewHabitService(repo repository.Repository) *HabitService {
	return &HabitService{repo: repo}
}

func (s *HabitService) CreateHabit(ctx context.Context, user *domain.User, name, description string, frequency domain.Frequency) (*domain.Habit, error) {
	count, err := s.repo.CountUserHabits(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("count habits: %w", err)
	}

	limit := domain.FreeHabitsLimit
	if user.HasActiveSubscription() {
		limit = domain.PremiumHabitsLimit
	}

	if count >= limit {
		return nil, ErrHabitLimitReached
	}

	habit := &domain.Habit{
		UserID:      user.ID,
		Name:        name,
		Description: description,
		Frequency:   frequency,
		IsActive:    true,
	}

	if err := s.repo.CreateHabit(ctx, habit); err != nil {
		return nil, fmt.Errorf("create habit: %w", err)
	}

	return habit, nil
}

func (s *HabitService) CreateHabitWithEmoji(ctx context.Context, user *domain.User, name, description string, frequency domain.Frequency, emoji string) (*domain.Habit, error) {
	count, err := s.repo.CountUserHabits(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	limit := domain.FreeHabitsLimit
	if user.HasActiveSubscription() {
		limit = domain.PremiumHabitsLimit
	}

	if count >= limit {
		return nil, ErrHabitLimitReached
	}

	habit := &domain.Habit{
		UserID:      user.ID,
		Name:        name,
		Description: description,
		Frequency:   frequency,
		IsActive:    true,
		Emoji:       emoji,
	}

	if err := s.repo.CreateHabit(ctx, habit); err != nil {
		return nil, err
	}

	return habit, nil
}

func (s *HabitService) GetUserHabits(ctx context.Context, userID int64) ([]*domain.Habit, error) {
	return s.repo.GetActiveHabits(ctx, userID)
}

func (s *HabitService) GetHabit(ctx context.Context, habitID int64) (*domain.Habit, error) {
	habit, err := s.repo.GetHabitByID(ctx, habitID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrHabitNotFound
	}
	return habit, err
}

func (s *HabitService) CompleteHabit(ctx context.Context, habitID, userID int64) error {
	habit, err := s.repo.GetHabitByID(ctx, habitID)
	if err != nil {
		return fmt.Errorf("get habit: %w", err)
	}

	if habit.UserID != userID {
		return ErrAccessDenied
	}

	log := &domain.HabitLog{
		HabitID:   habitID,
		UserID:    userID,
		Date:      time.Now(),
		Completed: true,
	}

	return s.repo.LogHabit(ctx, log)
}

func (s *HabitService) UncompleteHabit(ctx context.Context, habitID, userID int64) error {
	habit, err := s.repo.GetHabitByID(ctx, habitID)
	if err != nil {
		return fmt.Errorf("get habit: %w", err)
	}

	if habit.UserID != userID {
		return ErrAccessDenied
	}

	log := &domain.HabitLog{
		HabitID:   habitID,
		UserID:    userID,
		Date:      time.Now(),
		Completed: false,
	}

	return s.repo.LogHabit(ctx, log)
}

func (s *HabitService) GetTodayStatus(ctx context.Context, userID int64) (map[int64]bool, error) {
	logs, err := s.repo.GetUserLogsForDate(ctx, userID, time.Now())
	if err != nil {
		return nil, err
	}

	status := make(map[int64]bool)
	for _, log := range logs {
		status[log.HabitID] = log.Completed
	}

	return status, nil
}

func (s *HabitService) GetHabitStats(ctx context.Context, habitID int64) (*domain.HabitStats, error) {
	return s.repo.GetHabitStats(ctx, habitID)
}

func (s *HabitService) GetUserStats(ctx context.Context, userID int64) ([]*domain.HabitStats, error) {
	return s.repo.GetUserStats(ctx, userID)
}

func (s *HabitService) GetUserOverallStreak(ctx context.Context, userID int64) (int, error) {
	return s.repo.GetUserOverallStreak(ctx, userID)
}

func (s *HabitService) DeleteHabit(ctx context.Context, habitID, userID int64) error {
	habit, err := s.repo.GetHabitByID(ctx, habitID)
	if err != nil {
		return err
	}

	if habit.UserID != userID {
		return ErrAccessDenied
	}

	return s.repo.DeleteHabit(ctx, habitID)
}

func (s *HabitService) UpdateHabitReminder(ctx context.Context, habitID, userID int64, reminderTime *string) error {
	habit, err := s.repo.GetHabitByID(ctx, habitID)
	if err != nil {
		return err
	}

	if habit.UserID != userID {
		return ErrAccessDenied
	}

	habit.ReminderTime = reminderTime
	return s.repo.UpdateHabit(ctx, habit)
}
