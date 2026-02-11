package service

import (
	"context"
	"fmt"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

type AchievementResult struct {
	Achievement *domain.AchievementConfig
	IsNew       bool
	BonusDays   int
}

type AchievementService struct {
	repo   repository.Repository
	subSvc *SubscriptionService
}

func NewAchievementService(repo repository.Repository, subSvc *SubscriptionService) *AchievementService {
	return &AchievementService{repo: repo, subSvc: subSvc}
}

func (s *AchievementService) CheckAndUnlockAchievements(ctx context.Context, userID int64, currentStreak int) (*AchievementResult, error) {
	for _, cfg := range domain.AchievementsConfig {
		if currentStreak >= cfg.StreakDays {
			has, err := s.repo.HasAchievement(ctx, userID, cfg.Type)
			if err != nil {
				return nil, fmt.Errorf("check achievement: %w", err)
			}

			if !has {
				achievement := &domain.Achievement{
					UserID:     userID,
					Type:       cfg.Type,
					StreakDays: cfg.StreakDays,
					BonusDays:  cfg.BonusDays,
				}

				if err := s.repo.CreateAchievement(ctx, achievement); err != nil {
					return nil, fmt.Errorf("create achievement: %w", err)
				}

				if cfg.BonusDays > 0 {
					if err := s.subSvc.AddSubscriptionDays(ctx, userID, cfg.BonusDays); err != nil {
						return nil, fmt.Errorf("add bonus days: %w", err)
					}
				}

				return &AchievementResult{
					Achievement: &cfg,
					IsNew:       true,
					BonusDays:   cfg.BonusDays,
				}, nil
			}
		}
	}

	return nil, nil
}

func (s *AchievementService) GetUserAchievements(ctx context.Context, userID int64) ([]*domain.Achievement, error) {
	return s.repo.GetUserAchievements(ctx, userID)
}

func (s *AchievementService) GetNextAchievement(ctx context.Context, userID int64, currentStreak int) (*domain.AchievementConfig, int, error) {
	for _, cfg := range domain.AchievementsConfig {
		if cfg.StreakDays > currentStreak {
			has, err := s.repo.HasAchievement(ctx, userID, cfg.Type)
			if err != nil {
				return nil, 0, err
			}
			if !has {
				daysLeft := cfg.StreakDays - currentStreak
				return &cfg, daysLeft, nil
			}
		}
	}
	return nil, 0, nil
}
