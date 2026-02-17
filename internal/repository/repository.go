package repository

import (
	"context"
	"time"

	"habit-tracker-bot/internal/domain"
)

type UserExportData struct {
	User         *domain.User
	Habits       []*domain.Habit
	Logs         []*domain.HabitLog
	Achievements []*domain.Achievement
	Stats        []*domain.HabitStats
}

type Repository interface {
	// Users
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	GetUserByReferralCode(ctx context.Context, code string) (*domain.User, error)
	UpdateUser(ctx context.Context, user *domain.User) error
	UpdateSubscription(ctx context.Context, userID int64, endDate time.Time) error
	AddSubscriptionDays(ctx context.Context, userID int64, days int) error
	AddDiscount(ctx context.Context, userID int64, percent int) error
	IncrementActionCount(ctx context.Context, userID int64) (int, error)
	ResetActionCount(ctx context.Context, userID int64) error
	GetTotalUsersCount(ctx context.Context) (int, error)
	GetUsersForBroadcast(ctx context.Context, lastUserID int64, limit int) ([]int64, int64, error)
	GetUserIDByTelegramID(ctx context.Context, telegramID int64) (int64, error)

	// Habits
	CreateHabit(ctx context.Context, habit *domain.Habit) error
	GetHabitByID(ctx context.Context, id int64) (*domain.Habit, error)
	GetActiveHabits(ctx context.Context, userID int64) ([]*domain.Habit, error)
	UpdateHabit(ctx context.Context, habit *domain.Habit) error
	DeleteHabit(ctx context.Context, id int64) error
	CountUserHabits(ctx context.Context, userID int64) (int, error)
	ClearReminders(ctx context.Context, userID int64) error

	// Habit Logs
	LogHabit(ctx context.Context, log *domain.HabitLog) error
	GetUserLogsForDate(ctx context.Context, userID int64, date time.Time) ([]*domain.HabitLog, error)
	GetUserLogsForPeriod(ctx context.Context, userID int64, from, to time.Time) ([]*domain.HabitLog, error)

	// Statistics
	GetHabitStats(ctx context.Context, habitID int64) (*domain.HabitStats, error)
	GetUserStats(ctx context.Context, userID int64) ([]*domain.HabitStats, error)
	GetUserOverallStreak(ctx context.Context, userID int64) (int, error)

	// Reminders
	GetHabitsForReminder(ctx context.Context, timeStr string) ([]*domain.Habit, error)
	GetUserTelegramIDByHabitID(ctx context.Context, habitID int64) (int64, error)

	// Payments
	CreatePayment(ctx context.Context, payment *domain.Payment) error
	GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error)
	UpdatePaymentStatus(ctx context.Context, orderID string, status domain.PaymentStatus, tinkoffID string) error
	UpdatePaymentPaid(ctx context.Context, orderID string, paidAt time.Time) error
	GetUserPendingPayment(ctx context.Context, userID int64) (*domain.Payment, error)

	// Referrals
	CreateReferral(ctx context.Context, referral *domain.Referral) error
	GetReferralByReferredID(ctx context.Context, referredID int64) (*domain.Referral, error)
	GetReferralsByReferrerID(ctx context.Context, referrerID int64) ([]*domain.Referral, error)
	GetReferralStats(ctx context.Context, userID int64) (*domain.ReferralStats, error)
	UpdateReferralStage1(ctx context.Context, referralID int64, bonusDays int) error
	UpdateReferralStage2(ctx context.Context, referralID int64, bonusDays int) error
	UpdateReferralDiscount(ctx context.Context, referralID int64) error
	GetPendingStage2Referrals(ctx context.Context, referredID int64) (*domain.Referral, error)
	CountBonusReferrals(ctx context.Context, userID int64) (int, error)

	// Achievements
	CreateAchievement(ctx context.Context, achievement *domain.Achievement) error
	GetUserAchievements(ctx context.Context, userID int64) ([]*domain.Achievement, error)
	HasAchievement(ctx context.Context, userID int64, achievementType domain.AchievementType) (bool, error)
	// Ads
	CreateAd(ctx context.Context, ad *domain.Ad) error
	GetActiveAds(ctx context.Context) ([]*domain.Ad, error)
	GetAdByID(ctx context.Context, id int64) (*domain.Ad, error)
	GetAllAds(ctx context.Context) ([]*domain.Ad, error)
	UpdateAd(ctx context.Context, ad *domain.Ad) error
	DeleteAd(ctx context.Context, id int64) error
	IncrementAdViews(ctx context.Context, adID int64) error
	IncrementAdClicks(ctx context.Context, adID int64) error

	// Broadcasts
	CreateBroadcast(ctx context.Context, b *domain.Broadcast) error
	GetBroadcastByID(ctx context.Context, id int64) (*domain.Broadcast, error)
	GetAllBroadcasts(ctx context.Context) ([]*domain.Broadcast, error)
	GetRunningBroadcast(ctx context.Context) (*domain.Broadcast, error)
	UpdateBroadcastStatus(ctx context.Context, id int64, status domain.BroadcastStatus) error
	UpdateBroadcastProgress(ctx context.Context, id int64, sent, failed int, lastUserID int64) error
	StartBroadcast(ctx context.Context, id int64, totalUsers int) error
	CompleteBroadcast(ctx context.Context, id int64) error

	// Admins
	IsAdmin(ctx context.Context, telegramID int64) (bool, error)
	AddAdmin(ctx context.Context, telegramID int64) error

	// Export
	GetAllUserData(ctx context.Context, userID int64) (*UserExportData, error)

	// Promocodes
	CreatePromocode(ctx context.Context, code string, discount int, maxUses int) error
	GetAllPromocodes(ctx context.Context) ([]*domain.Promocode, error)
	GetPromocodeByCode(ctx context.Context, code string) (*domain.Promocode, error)
	DeletePromocode(ctx context.Context, code string) error
	TogglePromocode(ctx context.Context, code string) error
	HasUserUsedPromocode(ctx context.Context, userID int64, promocodeID int64) (bool, error)
	SetUserActivePromocode(ctx context.Context, userID int64, promocodeID int64) error
	GetUserActivePromocode(ctx context.Context, userID int64) (*domain.Promocode, error)
	ClearUserActivePromocode(ctx context.Context, userID int64) error
	IncrementPromocodeUsage(ctx context.Context, promocodeID int64, userID int64) error

	// Promo
	GetUsersForFirstPromo(ctx context.Context) ([]int64, error)
	MarkFirstPromoSent(ctx context.Context, userID int64) error
	GetUsersForWeeklyPromo(ctx context.Context) ([]int64, error)
	MarkWeeklyPromoSent(ctx context.Context, userID int64) error

	UpdateHabitReminder(ctx context.Context, habitID int64, reminderTime *string, reminderDays []int) error

	Close()
}
