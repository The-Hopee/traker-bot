package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ==================== USER ====================

type User struct {
	ID                     int64
	TelegramID             int64
	Username               string
	FirstName              string
	SubscriptionEnd        *time.Time
	IsPremium              bool
	Timezone               string
	ReferralCode           string
	ReferredBy             *int64
	DiscountPercent        int
	ActionCount            int
	SubscribedToBroadcasts bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func (u *User) HasActiveSubscription() bool {
	if u.SubscriptionEnd == nil {
		return false
	}
	return time.Now().Before(*u.SubscriptionEnd)
}

func GenerateReferralCode() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ==================== HABIT ====================

type Habit struct {
	ID           int64
	UserID       int64
	Name         string
	Description  string
	Frequency    Frequency
	ReminderTime *string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ReminderDays []int // [1,2,3,4,5] = пн-пт, [1,2,3,4,5,6,7] = все дни
	Emoji        string
}

type Frequency string

const (
	FrequencyDaily   Frequency = "daily"
	FrequencyWeekly  Frequency = "weekly"
	FrequencyMonthly Frequency = "monthly"
)

// ==================== HABIT LOG ====================

type HabitLog struct {
	ID        int64
	HabitID   int64
	UserID    int64
	Date      time.Time
	Completed bool
	Note      string
	CreatedAt time.Time
}

// ==================== STATISTICS ====================

type HabitStats struct {
	HabitID         int64
	HabitName       string
	TotalDays       int
	CompletedDays   int
	CurrentStreak   int
	BestStreak      int
	CompletionRate  float64
	LastCompletedAt *time.Time
}

// ==================== PAYMENT ====================

type Payment struct {
	ID              int64
	UserID          int64
	TinkoffID       string
	OrderID         string
	Amount          int64
	OriginalAmount  int64
	DiscountPercent int
	Status          PaymentStatus
	PaymentURL      string
	Description     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	PaidAt          *time.Time
}

type PaymentStatus string

const (
	PaymentStatusNew       PaymentStatus = "NEW"
	PaymentStatusPending   PaymentStatus = "PENDING"
	PaymentStatusConfirmed PaymentStatus = "CONFIRMED"
	PaymentStatusCanceled  PaymentStatus = "CANCELED"
	PaymentStatusRejected  PaymentStatus = "REJECTED"
	PaymentStatusRefunded  PaymentStatus = "REFUNDED"
)

// ==================== REFERRAL ====================

type Referral struct {
	ID              int64
	ReferrerID      int64
	ReferredID      int64
	ReferralCode    string
	Stage1Applied   bool
	Stage1BonusDays int
	Stage2Applied   bool
	Stage2BonusDays int
	GaveDiscount    bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ReferralStats struct {
	TotalReferrals      int
	BonusReferrals      int
	DiscountReferrals   int
	Stage1Completed     int
	Stage2Completed     int
	TotalBonusDays      int
	AccumulatedDiscount int
	CanInvite           bool
	CurrentStreak       int
	DaysUntilUnlock     int
}

// ==================== ACHIEVEMENTS ====================

type Achievement struct {
	ID         int64
	UserID     int64
	Type       AchievementType
	StreakDays int
	BonusDays  int
	UnlockedAt time.Time
}

type AchievementType string

const (
	AchievementStreak7   AchievementType = "streak_7"
	AchievementStreak14  AchievementType = "streak_14"
	AchievementStreak30  AchievementType = "streak_30"
	AchievementStreak60  AchievementType = "streak_60"
	AchievementStreak100 AchievementType = "streak_100"
)

type AchievementConfig struct {
	Type        AchievementType
	StreakDays  int
	BonusDays   int
	Title       string
	Description string
	Emoji       string
}

var AchievementsConfig = []AchievementConfig{
	{AchievementStreak7, 7, 0, "Первая неделя", "Реферальная программа разблокирована!", "🔓"},
	{AchievementStreak14, 14, 2, "Две недели", "+2 дня Premium", "🔥"},
	{AchievementStreak30, 30, 3, "Месяц силы", "+3 дня Premium", "💪"},
	{AchievementStreak60, 60, 5, "Два месяца", "+5 дней Premium", "⭐️"},
	{AchievementStreak100, 100, 7, "Легенда", "+7 дней Premium", "🏆"},
}

func GetAchievementConfig(t AchievementType) *AchievementConfig {
	for _, cfg := range AchievementsConfig {
		if cfg.Type == t {
			return &cfg
		}
	}
	return nil
}

// ==================== ADS ====================

type Ad struct {
	ID          int64
	Name        string
	Text        string
	ImageURL    *string
	ButtonText  *string
	ButtonURL   *string
	IsActive    bool
	Priority    int
	ViewsCount  int
	ClicksCount int
	StartDate   *time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
}

// ==================== BROADCASTS ====================

type Broadcast struct {
	ID          int64
	Name        string
	Text        string
	ImageURL    *string
	ButtonText  *string
	ButtonURL   *string
	Status      BroadcastStatus
	TotalUsers  int
	SentCount   int
	FailedCount int
	LastUserID  int64
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

type BroadcastStatus string

const (
	BroadcastDraft     BroadcastStatus = "draft"
	BroadcastRunning   BroadcastStatus = "running"
	BroadcastPaused    BroadcastStatus = "paused"
	BroadcastCompleted BroadcastStatus = "completed"
)

// ==================== PROMOCODE ====================
type Promocode struct {
	ID              int64
	Code            string
	DiscountPercent int
	MaxUses         *int
	UsedCount       int
	IsActive        bool
	CreatedAt       time.Time
}

// ==================== TINKOFF ====================

type TinkoffInitRequest struct {
	TerminalKey string              `json:"TerminalKey"`
	Amount      int64               `json:"Amount"`
	OrderId     string              `json:"OrderId"`
	Description string              `json:"Description,omitempty"`
	Token       string              `json:"Token"`
	DATA        *TinkoffPaymentData `json:"DATA,omitempty"`
}

type TinkoffPaymentData struct {
	TelegramUserID string `json:"TelegramUserID,omitempty"`
}

type TinkoffInitResponse struct {
	Success     bool   `json:"Success"`
	ErrorCode   string `json:"ErrorCode"`
	Message     string `json:"Message,omitempty"`
	TerminalKey string `json:"TerminalKey,omitempty"`
	Status      string `json:"Status,omitempty"`
	PaymentId   string `json:"PaymentId,omitempty"`
	OrderId     string `json:"OrderId,omitempty"`
	Amount      int64  `json:"Amount,omitempty"`
	PaymentURL  string `json:"PaymentURL,omitempty"`
}

type TinkoffNotification struct {
	TerminalKey string `json:"TerminalKey"`
	OrderId     string `json:"OrderId"`
	Success     bool   `json:"Success"`
	Status      string `json:"Status"`
	PaymentId   int64  `json:"PaymentId"`
	ErrorCode   string `json:"ErrorCode"`
	Amount      int64  `json:"Amount"`
	Token       string `json:"Token"`
}

// GenerateTinkoffToken рассчитывает токен по официальному алгоритму Tinkoff:
// 1. Берём все поля запроса (кроме Token).
// 2. Добавляем поле Password с секретным ключом терминала.
// 3. Сортируем имена полей по алфавиту.
// 4. Конкатенируем значения в одну строку без разделителей.
// 5. Считаем SHA-256 от полученной строки.
func GenerateTinkoffToken(params map[string]string, password string) string {
	// Копируем параметры, чтобы не мутировать исходную map
	data := make(map[string]string, len(params)+1)
	for k, v := range params {
		if k == "Token" {
			continue
		}
		data[k] = v
	}
	// Добавляем Password как отдельное поле
	data["Password"] = password

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	values := make([]string, 0, len(keys))
	for _, k := range keys {
		values = append(values, data[k])
	}

	hash := sha256.Sum256([]byte(strings.Join(values, "")))
	return hex.EncodeToString(hash[:])
}

func VerifyTinkoffToken(n *TinkoffNotification, password string) bool {
	params := map[string]string{
		"TerminalKey": n.TerminalKey,
		"OrderId":     n.OrderId,
		"Success":     fmt.Sprintf("%t", n.Success),
		"Status":      n.Status,
		"PaymentId":   fmt.Sprintf("%d", n.PaymentId),
		"ErrorCode":   n.ErrorCode,
		"Amount":      fmt.Sprintf("%d", n.Amount),
	}
	return n.Token == GenerateTinkoffToken(params, password)
}

// GenerateTinkoffTokenForGetState генерирует токен для GetState с фиксированным порядком полей (без Password)
func GenerateTinkoffTokenForGetState(paymentID, terminalKey, password string) string {
	// Порядок строго: paymentID + Password + TerminalKey
	values := []string{password, paymentID, terminalKey}
	hash := sha256.Sum256([]byte(strings.Join(values, "")))
	return hex.EncodeToString(hash[:])
}

// ==================== CONSTANTS ====================

const (
	FreeHabitsLimit    = 3
	PremiumHabitsLimit = 100
	FreeHistoryDays    = 7
	PremiumHistoryDays = 365

	ReferralStage1Bonus    = 7
	ReferralStage2Bonus    = 7
	ReferralUnlockStreak   = 3
	ReferralStage2Streak   = 7
	ReferralBonusLimit     = 5
	ReferralDiscountPerRef = 25
	MaxReferralDiscount    = 50
	SubscriptionDays       = 30
	AdFrequency            = 5
)
