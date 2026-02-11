package service

import (
	"context"
	"errors"
	"fmt"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

var (
	ErrReferralNotUnlocked = errors.New("Реферальная система не разблокирована")
	ErrCannotReferSelf     = errors.New("Нельзя пригласить самого себя")
	ErrAlreadyReferred     = errors.New("Пользователь уже был приглашен")
	ErrInvalidReferralCode = errors.New("Недействительный реферальный код")
)

type ReferralResult struct {
	Stage          int
	ReferrerBonus  int
	ReferredBonus  int
	IsDiscount     bool
	ReferrerUserID int64
}

type ReferralService struct {
	repo   repository.Repository
	subSvc *SubscriptionService
}

func NewReferralService(repo repository.Repository, subSvc *SubscriptionService) *ReferralService {
	return &ReferralService{repo: repo, subSvc: subSvc}
}

func (s *ReferralService) CanUserInvite(ctx context.Context, userID int64) (bool, int, error) {
	streak, err := s.repo.GetUserOverallStreak(ctx, userID)
	if err != nil {
		return false, 0, err
	}
	return streak >= domain.ReferralUnlockStreak, streak, nil
}

func (s *ReferralService) GetReferralLink(ctx context.Context, userID int64, botUsername string) (string, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://t.me/%s?start=ref_%s", botUsername, user.ReferralCode), nil
}

func (s *ReferralService) GetReferralStats(ctx context.Context, userID int64) (*domain.ReferralStats, error) {
	return s.repo.GetReferralStats(ctx, userID)
}

func (s *ReferralService) ProcessReferralStage1(ctx context.Context, referralCode string, newUser *domain.User) (*ReferralResult, error) {
	referrer, err := s.repo.GetUserByReferralCode(ctx, referralCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidReferralCode
		}
		return nil, fmt.Errorf("get referrer: %w", err)
	}

	if referrer.TelegramID == newUser.TelegramID {
		return nil, ErrCannotReferSelf
	}

	canInvite, _, err := s.CanUserInvite(ctx, referrer.ID)
	if err != nil {
		return nil, fmt.Errorf("check can invite: %w", err)
	}
	if !canInvite {
		return nil, ErrReferralNotUnlocked
	}

	existingRef, err := s.repo.GetReferralByReferredID(ctx, newUser.ID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check existing: %w", err)
	}
	if existingRef != nil {
		return nil, ErrAlreadyReferred
	}

	bonusCount, _ := s.repo.CountBonusReferrals(ctx, referrer.ID)
	isOverLimit := bonusCount >= domain.ReferralBonusLimit

	var referral *domain.Referral
	var result *ReferralResult

	if isOverLimit {
		referral = &domain.Referral{
			ReferrerID:      referrer.ID,
			ReferredID:      newUser.ID,
			ReferralCode:    referralCode,
			Stage1Applied:   true,
			Stage1BonusDays: 0,
			GaveDiscount:    true,
		}

		if err := s.repo.CreateReferral(ctx, referral); err != nil {
			return nil, fmt.Errorf("create referral: %w", err)
		}

		if err := s.repo.AddDiscount(ctx, referrer.ID, domain.ReferralDiscountPerRef); err != nil {
			return nil, fmt.Errorf("add discount: %w", err)
		}

		if err := s.subSvc.AddSubscriptionDays(ctx, newUser.ID, domain.ReferralStage1Bonus); err != nil {
			return nil, fmt.Errorf("add bonus to referred: %w", err)
		}

		result = &ReferralResult{
			Stage:          1,
			ReferrerBonus:  domain.ReferralDiscountPerRef,
			ReferredBonus:  domain.ReferralStage1Bonus,
			IsDiscount:     true,
			ReferrerUserID: referrer.ID,
		}
	} else {
		referral = &domain.Referral{
			ReferrerID:      referrer.ID,
			ReferredID:      newUser.ID,
			ReferralCode:    referralCode,
			Stage1Applied:   true,
			Stage1BonusDays: domain.ReferralStage1Bonus,
			GaveDiscount:    false,
		}

		if err := s.repo.CreateReferral(ctx, referral); err != nil {
			return nil, fmt.Errorf("create referral: %w", err)
		}
		if err := s.subSvc.AddSubscriptionDays(ctx, referrer.ID, domain.ReferralStage1Bonus); err != nil {
			return nil, fmt.Errorf("add bonus to referrer: %w", err)
		}
		if err := s.subSvc.AddSubscriptionDays(ctx, newUser.ID, domain.ReferralStage1Bonus); err != nil {
			return nil, fmt.Errorf("add bonus to referred: %w", err)
		}

		result = &ReferralResult{
			Stage:          1,
			ReferrerBonus:  domain.ReferralStage1Bonus,
			ReferredBonus:  domain.ReferralStage1Bonus,
			IsDiscount:     false,
			ReferrerUserID: referrer.ID,
		}
	}

	return result, nil
}

func (s *ReferralService) ProcessReferralStage2(ctx context.Context, referredUserID int64, currentStreak int) (*ReferralResult, error) {
	if currentStreak < domain.ReferralStage2Streak {
		return nil, nil
	}

	referral, err := s.repo.GetPendingStage2Referrals(ctx, referredUserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get pending: %w", err)
	}

	if !referral.GaveDiscount {
		if err := s.repo.UpdateReferralStage2(ctx, referral.ID, domain.ReferralStage2Bonus); err != nil {
			return nil, fmt.Errorf("update stage2: %w", err)
		}

		if err := s.subSvc.AddSubscriptionDays(ctx, referral.ReferrerID, domain.ReferralStage2Bonus); err != nil {
			return nil, fmt.Errorf("add bonus to referrer: %w", err)
		}
		if err := s.subSvc.AddSubscriptionDays(ctx, referredUserID, domain.ReferralStage2Bonus); err != nil {
			return nil, fmt.Errorf("add bonus to referred: %w", err)
		}

		return &ReferralResult{
			Stage:          2,
			ReferrerBonus:  domain.ReferralStage2Bonus,
			ReferredBonus:  domain.ReferralStage2Bonus,
			IsDiscount:     false,
			ReferrerUserID: referral.ReferrerID,
		}, nil
	}

	return nil, nil
}

func (s *ReferralService) GetUserReferrals(ctx context.Context, userID int64) ([]*domain.Referral, error) {
	return s.repo.GetReferralsByReferrerID(ctx, userID)
}

func (s *ReferralService) GetReferrerInfo(ctx context.Context, userID int64) (*domain.User, error) {
	referral, err := s.repo.GetReferralByReferredID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return s.repo.GetUserByID(ctx, referral.ReferrerID)
}
