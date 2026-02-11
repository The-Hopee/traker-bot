package service

import (
	"context"
	"time"

	"habit-tracker-bot/internal/repository"
)

type SubscriptionService struct {
	repo  repository.Repository
	price int64
}

func NewSubscriptionService(repo repository.Repository, price int64) *SubscriptionService {
	return &SubscriptionService{repo: repo, price: price}
}

func (s *SubscriptionService) GetPrice() int64 {
	return s.price
}

func (s *SubscriptionService) GetPriceWithDiscount(discount int) int64 {
	if discount <= 0 {
		return s.price
	}
	return s.price * int64(100-discount) / 100
}

func (s *SubscriptionService) GetPriceRubles() float64 {
	return float64(s.price) / 100
}

func (s *SubscriptionService) AddSubscriptionDays(ctx context.Context, userID int64, days int) error {
	return s.repo.AddSubscriptionDays(ctx, userID, days)
}

func (s *SubscriptionService) SetSubscriptionEnd(ctx context.Context, userID int64, endDate time.Time) error {
	return s.repo.UpdateSubscription(ctx, userID, endDate)
}

func (s *SubscriptionService) IsSubscribed(ctx context.Context, userID int64) (bool, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.HasActiveSubscription(), nil
}
