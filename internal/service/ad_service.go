package service

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"habit-tracker-bot/internal/domain"
	"habit-tracker-bot/internal/repository"
)

type AdService struct {
	repo       repository.Repository
	cache      []*domain.Ad
	lastUpdate time.Time
	mu         sync.RWMutex
}

func NewAdService(repo repository.Repository) *AdService {
	return &AdService{repo: repo}
}

func (s *AdService) RefreshCache(ctx context.Context) error {
	ads, err := s.repo.GetActiveAds(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cache = ads
	s.lastUpdate = time.Now()
	s.mu.Unlock()
	return nil
}

func (s *AdService) ShouldShowAd(ctx context.Context, userID int64) (bool, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if user.HasActiveSubscription() {
		return false, nil
	}

	count, err := s.repo.IncrementActionCount(ctx, userID)
	if err != nil {
		return false, err
	}

	if count >= domain.AdFrequency {
		s.repo.ResetActionCount(ctx, userID)
		return true, nil
	}

	return false, nil
}

func (s *AdService) GetRandomAd(ctx context.Context) *domain.Ad {
	s.mu.RLock()
	needRefresh := time.Since(s.lastUpdate) > 5*time.Minute || len(s.cache) == 0
	s.mu.RUnlock()

	if needRefresh {
		s.RefreshCache(ctx)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.cache) == 0 {
		return nil
	}

	totalWeight := 0
	for _, ad := range s.cache {
		totalWeight += ad.Priority + 1
	}

	r := rand.Intn(totalWeight)
	for _, ad := range s.cache {
		r -= ad.Priority + 1
		if r < 0 {
			return ad
		}
	}

	return s.cache[0]
}

func (s *AdService) TrackView(ctx context.Context, adID int64) {
	s.repo.IncrementAdViews(ctx, adID)
}

func (s *AdService) TrackClick(ctx context.Context, adID int64) {
	s.repo.IncrementAdClicks(ctx, adID)
}
