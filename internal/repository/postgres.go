package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"habit-tracker-bot/internal/domain"
)

var ErrNotFound = errors.New("not found")

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(ctx context.Context, databaseURL string) (*PostgresRepository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 5

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresRepository{db: pool}, nil
}

func (r *PostgresRepository) Close() {
	r.db.Close()
}

// ==================== USERS ====================

func (r *PostgresRepository) CreateUser(ctx context.Context, user *domain.User) error {
	if user.ReferralCode == "" {
		user.ReferralCode = domain.GenerateReferralCode()
	}

	query := `
    INSERT INTO users (telegram_id, username, first_name, timezone, referral_code, referred_by, discount_percent, subscribed_to_broadcasts, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8, $8)
    ON CONFLICT (telegram_id) DO UPDATE SET
      username = EXCLUDED.username,
      first_name = EXCLUDED.first_name,
      updated_at = EXCLUDED.updated_at
    RETURNING id, referral_code, discount_percent`

	return r.db.QueryRow(ctx, query,
		user.TelegramID, user.Username, user.FirstName, user.Timezone,
		user.ReferralCode, user.ReferredBy, user.DiscountPercent, time.Now(),
	).Scan(&user.ID, &user.ReferralCode, &user.DiscountPercent)
}

func (r *PostgresRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	query := `
    SELECT id, telegram_id, username, first_name, subscription_end, timezone, 
           referral_code, referred_by, discount_percent, action_count, 
           subscribed_to_broadcasts, created_at, updated_at
    FROM users WHERE telegram_id = $1`

	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.SubscriptionEnd, &user.Timezone, &user.ReferralCode,
		&user.ReferredBy, &user.DiscountPercent, &user.ActionCount,
		&user.SubscribedToBroadcasts, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.IsPremium = user.HasActiveSubscription()
	return user, nil
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `
    SELECT id, telegram_id, username, first_name, subscription_end, timezone, 
           referral_code, referred_by, discount_percent, action_count,
           subscribed_to_broadcasts, created_at, updated_at
    FROM users WHERE id = $1`

	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.SubscriptionEnd, &user.Timezone, &user.ReferralCode,
		&user.ReferredBy, &user.DiscountPercent, &user.ActionCount,
		&user.SubscribedToBroadcasts, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.IsPremium = user.HasActiveSubscription()
	return user, nil
}

func (r *PostgresRepository) GetUserByReferralCode(ctx context.Context, code string) (*domain.User, error) {
	query := `
    SELECT id, telegram_id, username, first_name, subscription_end, timezone, 
           referral_code, referred_by, discount_percent, action_count,
           subscribed_to_broadcasts, created_at, updated_at
    FROM users WHERE referral_code = $1`
	user := &domain.User{}
	err := r.db.QueryRow(ctx, query, code).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName,
		&user.SubscriptionEnd, &user.Timezone, &user.ReferralCode,
		&user.ReferredBy, &user.DiscountPercent, &user.ActionCount,
		&user.SubscribedToBroadcasts, &user.CreatedAt, &user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	user.IsPremium = user.HasActiveSubscription()
	return user, nil
}

func (r *PostgresRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET username=$2, first_name=$3, timezone=$4, updated_at=$5 WHERE id=$1`
	_, err := r.db.Exec(ctx, query, user.ID, user.Username, user.FirstName, user.Timezone, time.Now())
	return err
}

func (r *PostgresRepository) UpdateSubscription(ctx context.Context, userID int64, endDate time.Time) error {
	query := `UPDATE users SET subscription_end=$2, updated_at=$3 WHERE id=$1`
	_, err := r.db.Exec(ctx, query, userID, endDate, time.Now())
	return err
}

func (r *PostgresRepository) AddSubscriptionDays(ctx context.Context, userID int64, days int) error {
	query := `
    UPDATE users SET 
      subscription_end = CASE 
        WHEN subscription_end IS NULL OR subscription_end < NOW() 
        THEN NOW() + INTERVAL '1 day' * $2
        ELSE subscription_end + INTERVAL '1 day' * $2
      END,
      updated_at = $3
    WHERE id = $1`
	_, err := r.db.Exec(ctx, query, userID, days, time.Now())
	return err
}

func (r *PostgresRepository) AddDiscount(ctx context.Context, userID int64, percent int) error {
	query := `UPDATE users SET discount_percent = LEAST(discount_percent + $2, $3), updated_at = $4 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, userID, percent, domain.MaxReferralDiscount, time.Now())
	return err
}

func (r *PostgresRepository) IncrementActionCount(ctx context.Context, userID int64) (int, error) {
	query := `UPDATE users SET action_count = action_count + 1, updated_at = $2 WHERE id = $1 RETURNING action_count`
	var count int
	err := r.db.QueryRow(ctx, query, userID, time.Now()).Scan(&count)
	return count, err
}

func (r *PostgresRepository) ResetActionCount(ctx context.Context, userID int64) error {
	query := `UPDATE users SET action_count = 0, updated_at = $2 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, userID, time.Now())
	return err
}

func (r *PostgresRepository) GetTotalUsersCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE subscribed_to_broadcasts = true`).Scan(&count)
	return count, err
}

func (r *PostgresRepository) GetUsersForBroadcast(ctx context.Context, lastUserID int64, limit int) ([]int64, int64, error) {
	query := `
    SELECT id, telegram_id FROM users 
    WHERE id > $1 AND subscribed_to_broadcasts = true
    ORDER BY id ASC LIMIT $2`

	rows, err := r.db.Query(ctx, query, lastUserID, limit)
	if err != nil {
		return nil, lastUserID, err
	}
	defer rows.Close()

	var telegramIDs []int64
	var maxID int64 = lastUserID
	for rows.Next() {
		var id, telegramID int64
		if err := rows.Scan(&id, &telegramID); err != nil {
			return nil, lastUserID, err
		}
		telegramIDs = append(telegramIDs, telegramID)
		if id > maxID {
			maxID = id
		}
	}
	return telegramIDs, maxID, nil
}

func (r *PostgresRepository) GetUserIDByTelegramID(ctx context.Context, telegramID int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT id FROM users WHERE telegram_id = $1`, telegramID).Scan(&id)
	return id, err
}

// ==================== HABITS ====================
func (r *PostgresRepository) CreateHabit(ctx context.Context, habit *domain.Habit) error {
	query := `
	  INSERT INTO habits (user_id, name, description, frequency, reminder_time, is_active, created_at, updated_at)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $7) RETURNING id`
	return r.db.QueryRow(ctx, query,
		habit.UserID, habit.Name, habit.Description, habit.Frequency,
		habit.ReminderTime, habit.IsActive, time.Now(),
	).Scan(&habit.ID)
}

func (r *PostgresRepository) GetHabitByID(ctx context.Context, id int64) (*domain.Habit, error) {
	query := `
	  SELECT id, user_id, name, description, frequency, reminder_time, is_active, created_at, updated_at
	  FROM habits WHERE id = $1`

	habit := &domain.Habit{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&habit.ID, &habit.UserID, &habit.Name, &habit.Description,
		&habit.Frequency, &habit.ReminderTime, &habit.IsActive,
		&habit.CreatedAt, &habit.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return habit, err
}

func (r *PostgresRepository) GetActiveHabits(ctx context.Context, userID int64) ([]*domain.Habit, error) {
	query := `
	  SELECT id, user_id, name, description, frequency, reminder_time, is_active, created_at, updated_at
	  FROM habits WHERE user_id = $1 AND is_active = true ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var habits []*domain.Habit
	for rows.Next() {
		h := &domain.Habit{}
		if err := rows.Scan(&h.ID, &h.UserID, &h.Name, &h.Description, &h.Frequency, &h.ReminderTime, &h.IsActive, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		habits = append(habits, h)
	}
	return habits, nil
}

func (r *PostgresRepository) UpdateHabit(ctx context.Context, habit *domain.Habit) error {
	query := `UPDATE habits SET name=$2, description=$3, frequency=$4, reminder_time=$5, is_active=$6, updated_at=$7 WHERE id=$1`
	_, err := r.db.Exec(ctx, query, habit.ID, habit.Name, habit.Description, habit.Frequency, habit.ReminderTime, habit.IsActive, time.Now())
	return err
}

func (r *PostgresRepository) DeleteHabit(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE habits SET is_active = false, updated_at = $2 WHERE id = $1`, id, time.Now())
	return err
}

func (r *PostgresRepository) CountUserHabits(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM habits WHERE user_id = $1 AND is_active = true`, userID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) ClearReminders(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE habits SET reminder_time = NULL, updated_at = $2 WHERE user_id = $1`, userID, time.Now())
	return err
}

// ==================== HABIT LOGS ====================

func (r *PostgresRepository) LogHabit(ctx context.Context, log *domain.HabitLog) error {
	query := `
	  INSERT INTO habit_logs (habit_id, user_id, date, completed, note, created_at)
	  VALUES ($1, $2, $3, $4, $5, $6)
	  ON CONFLICT (habit_id, date) DO UPDATE SET completed = EXCLUDED.completed, note = EXCLUDED.note
	  RETURNING id`
	return r.db.QueryRow(ctx, query, log.HabitID, log.UserID, log.Date.Truncate(24*time.Hour), log.Completed, log.Note, time.Now()).Scan(&log.ID)
}

func (r *PostgresRepository) GetUserLogsForDate(ctx context.Context, userID int64, date time.Time) ([]*domain.HabitLog, error) {
	query := `
	  SELECT hl.id, hl.habit_id, hl.user_id, hl.date, hl.completed, hl.note, hl.created_at
	  FROM habit_logs hl JOIN habits h ON h.id = hl.habit_id
	  WHERE h.user_id = $1 AND hl.date = $2 AND h.is_active = true`

	rows, err := r.db.Query(ctx, query, userID, date.Truncate(24*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []*domain.HabitLog
	for rows.Next() {
		l := &domain.HabitLog{}
		if err := rows.Scan(&l.ID, &l.HabitID, &l.UserID, &l.Date, &l.Completed, &l.Note, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (r *PostgresRepository) GetUserLogsForPeriod(ctx context.Context, userID int64, from, to time.Time) ([]*domain.HabitLog, error) {
	query := `
    SELECT hl.id, hl.habit_id, hl.user_id, hl.date, hl.completed, hl.note, hl.created_at
    FROM habit_logs hl JOIN habits h ON h.id = hl.habit_id
    WHERE h.user_id = $1 AND hl.date >= $2 AND hl.date <= $3 ORDER BY hl.date DESC`

	rows, err := r.db.Query(ctx, query, userID, from.Truncate(24*time.Hour), to.Truncate(24*time.Hour))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.HabitLog
	for rows.Next() {
		l := &domain.HabitLog{}
		if err := rows.Scan(&l.ID, &l.HabitID, &l.UserID, &l.Date, &l.Completed, &l.Note, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// ==================== STATISTICS ====================

func (r *PostgresRepository) GetHabitStats(ctx context.Context, habitID int64) (*domain.HabitStats, error) {
	query := `
    SELECT h.id, h.name,
      COUNT(hl.id) as total_days,
      COUNT(hl.id) FILTER (WHERE hl.completed = true) as completed_days,
      MAX(hl.date) FILTER (WHERE hl.completed = true) as last_completed
    FROM habits h LEFT JOIN habit_logs hl ON hl.habit_id = h.id
    WHERE h.id = $1 GROUP BY h.id, h.name`

	stats := &domain.HabitStats{}
	err := r.db.QueryRow(ctx, query, habitID).Scan(&stats.HabitID, &stats.HabitName, &stats.TotalDays, &stats.CompletedDays, &stats.LastCompletedAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if stats.TotalDays > 0 {
		stats.CompletionRate = float64(stats.CompletedDays) / float64(stats.TotalDays) * 100
	}
	stats.CurrentStreak = r.calculateCurrentStreak(ctx, habitID)
	stats.BestStreak = r.calculateBestStreak(ctx, habitID)
	return stats, nil
}

func (r *PostgresRepository) calculateCurrentStreak(ctx context.Context, habitID int64) int {
	query := `
    WITH RECURSIVE streak AS (
      SELECT date, 1 as cnt FROM habit_logs WHERE habit_id = $1 AND completed = true AND date = CURRENT_DATE
      UNION ALL
      SELECT hl.date, s.cnt + 1 FROM habit_logs hl JOIN streak s ON hl.date = s.date - 1
      WHERE hl.habit_id = $1 AND hl.completed = true
    )
    SELECT COALESCE(MAX(cnt), 0) FROM streak`

	var streak int
	r.db.QueryRow(ctx, query, habitID).Scan(&streak)
	if streak == 0 {
		query = `
      WITH RECURSIVE streak AS (
        SELECT date, 1 as cnt FROM habit_logs WHERE habit_id = $1 AND completed = true AND date = CURRENT_DATE - 1
        UNION ALL
        SELECT hl.date, s.cnt + 1 FROM habit_logs hl JOIN streak s ON hl.date = s.date - 1
        WHERE hl.habit_id = $1 AND hl.completed = true
      )
      SELECT COALESCE(MAX(cnt), 0) FROM streak`
		r.db.QueryRow(ctx, query, habitID).Scan(&streak)
	}
	return streak
}

func (r *PostgresRepository) calculateBestStreak(ctx context.Context, habitID int64) int {
	query := `
    WITH streaks AS (
      SELECT date, date - (ROW_NUMBER() OVER (ORDER BY date))::int AS grp
      FROM habit_logs WHERE habit_id = $1 AND completed = true
    )
    SELECT COALESCE(MAX(cnt), 0) FROM (SELECT COUNT(*) as cnt FROM streaks GROUP BY grp) t`

	var best int
	r.db.QueryRow(ctx, query, habitID).Scan(&best)
	return best
}

func (r *PostgresRepository) GetUserStats(ctx context.Context, userID int64) ([]*domain.HabitStats, error) {
	habits, err := r.GetActiveHabits(ctx, userID)
	if err != nil {
		return nil, err
	}
	var stats []*domain.HabitStats
	for _, h := range habits {
		s, _ := r.GetHabitStats(ctx, h.ID)
		if s != nil {
			stats = append(stats, s)
		}
	}
	return stats, nil
}
func (r *PostgresRepository) GetUserOverallStreak(ctx context.Context, userID int64) (int, error) {
	query := `
	  WITH user_habits AS (SELECT id FROM habits WHERE user_id = $1 AND is_active = true),
	  habit_count AS (SELECT COUNT(*) as total FROM user_habits),
	  daily_completions AS (
		SELECT hl.date, COUNT(DISTINCT hl.habit_id) as completed_count
		FROM habit_logs hl JOIN user_habits uh ON uh.id = hl.habit_id
		WHERE hl.completed = true GROUP BY hl.date
	  ),
	  full_days AS (
		SELECT dc.date FROM daily_completions dc, habit_count hc
		WHERE dc.completed_count >= hc.total AND hc.total > 0
	  ),
	  streak AS (
		SELECT date, date - (ROW_NUMBER() OVER (ORDER BY date DESC))::int AS grp
		FROM full_days WHERE date >= CURRENT_DATE - 365
	  )
	  SELECT COALESCE(COUNT(*), 0) FROM streak
	  WHERE grp = (SELECT grp FROM streak WHERE date = CURRENT_DATE OR date = CURRENT_DATE - 1 LIMIT 1)`

	var streak int
	err := r.db.QueryRow(ctx, query, userID).Scan(&streak)
	if err != nil {
		return 0, nil
	}
	return streak, nil
}

// ==================== REMINDERS ====================

func (r *PostgresRepository) GetHabitsForReminder(ctx context.Context, timeStr string) ([]*domain.Habit, error) {
	query := `
	  SELECT h.id, h.user_id, h.name, h.description, h.frequency, h.reminder_time, h.is_active, h.created_at, h.updated_at
	  FROM habits h JOIN users u ON u.id = h.user_id
	  WHERE h.reminder_time = $1 AND h.is_active = true AND u.subscription_end > NOW()
	  AND NOT EXISTS (SELECT 1 FROM habit_logs WHERE habit_id = h.id AND date = CURRENT_DATE AND completed = true)`

	rows, err := r.db.Query(ctx, query, timeStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var habits []*domain.Habit
	for rows.Next() {
		h := &domain.Habit{}
		if err := rows.Scan(&h.ID, &h.UserID, &h.Name, &h.Description, &h.Frequency, &h.ReminderTime, &h.IsActive, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, err
		}
		habits = append(habits, h)
	}
	return habits, nil
}

func (r *PostgresRepository) GetUserTelegramIDByHabitID(ctx context.Context, habitID int64) (int64, error) {
	var telegramID int64
	err := r.db.QueryRow(ctx, `SELECT u.telegram_id FROM users u JOIN habits h ON h.user_id = u.id WHERE h.id = $1`, habitID).Scan(&telegramID)
	return telegramID, err
}

// ==================== PAYMENTS ====================

func (r *PostgresRepository) CreatePayment(ctx context.Context, p *domain.Payment) error {
	query := `
	  INSERT INTO payments (user_id, tinkoff_id, order_id, amount, original_amount, discount_percent, status, payment_url, description, created_at, updated_at)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10) RETURNING id`
	return r.db.QueryRow(ctx, query, p.UserID, p.TinkoffID, p.OrderID, p.Amount, p.OriginalAmount, p.DiscountPercent, p.Status, p.PaymentURL, p.Description, time.Now()).Scan(&p.ID)
}

func (r *PostgresRepository) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	query := `
	  SELECT id, user_id, tinkoff_id, order_id, amount, original_amount, discount_percent, status, payment_url, description, created_at, updated_at, paid_at
	  FROM payments WHERE order_id = $1`

	p := &domain.Payment{}
	err := r.db.QueryRow(ctx, query, orderID).Scan(&p.ID, &p.UserID, &p.TinkoffID, &p.OrderID, &p.Amount, &p.OriginalAmount, &p.DiscountPercent, &p.Status, &p.PaymentURL, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (r *PostgresRepository) UpdatePaymentStatus(ctx context.Context, orderID string, status domain.PaymentStatus, tinkoffID string) error {
	_, err := r.db.Exec(ctx, `UPDATE payments SET status=$2, tinkoff_id=$3, updated_at=$4 WHERE order_id=$1`, orderID, status, tinkoffID, time.Now())
	return err
}
func (r *PostgresRepository) UpdatePaymentPaid(ctx context.Context, orderID string, paidAt time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE payments SET status=$2, paid_at=$3, updated_at=$4 WHERE order_id=$1`, orderID, domain.PaymentStatusConfirmed, paidAt, time.Now())
	return err
}

func (r *PostgresRepository) GetUserPendingPayment(ctx context.Context, userID int64) (*domain.Payment, error) {
	query := `
	  SELECT id, user_id, tinkoff_id, order_id, amount, original_amount, discount_percent, status, payment_url, description, created_at, updated_at, paid_at
	  FROM payments WHERE user_id = $1 AND status IN ('NEW', 'PENDING') ORDER BY created_at DESC LIMIT 1`

	p := &domain.Payment{}
	err := r.db.QueryRow(ctx, query, userID).Scan(&p.ID, &p.UserID, &p.TinkoffID, &p.OrderID, &p.Amount, &p.OriginalAmount, &p.DiscountPercent, &p.Status, &p.PaymentURL, &p.Description, &p.CreatedAt, &p.UpdatedAt, &p.PaidAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

// ==================== REFERRALS ====================

func (r *PostgresRepository) CreateReferral(ctx context.Context, ref *domain.Referral) error {
	query := `
	  INSERT INTO referrals (referrer_id, referred_id, referral_code, stage1_applied, stage1_bonus_days, stage2_applied, stage2_bonus_days, gave_discount, created_at, updated_at)
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
	  ON CONFLICT (referred_id) DO NOTHING RETURNING id`
	err := r.db.QueryRow(ctx, query, ref.ReferrerID, ref.ReferredID, ref.ReferralCode, ref.Stage1Applied, ref.Stage1BonusDays, ref.Stage2Applied, ref.Stage2BonusDays, ref.GaveDiscount, time.Now()).Scan(&ref.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (r *PostgresRepository) GetReferralByReferredID(ctx context.Context, referredID int64) (*domain.Referral, error) {
	query := `
	  SELECT id, referrer_id, referred_id, referral_code, stage1_applied, stage1_bonus_days, stage2_applied, stage2_bonus_days, gave_discount, created_at, updated_at
	  FROM referrals WHERE referred_id = $1`

	ref := &domain.Referral{}
	err := r.db.QueryRow(ctx, query, referredID).Scan(&ref.ID, &ref.ReferrerID, &ref.ReferredID, &ref.ReferralCode, &ref.Stage1Applied, &ref.Stage1BonusDays, &ref.Stage2Applied, &ref.Stage2BonusDays, &ref.GaveDiscount, &ref.CreatedAt, &ref.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ref, err
}

func (r *PostgresRepository) GetReferralsByReferrerID(ctx context.Context, referrerID int64) ([]*domain.Referral, error) {
	query := `
	  SELECT id, referrer_id, referred_id, referral_code, stage1_applied, stage1_bonus_days, stage2_applied, stage2_bonus_days, gave_discount, created_at, updated_at
	  FROM referrals WHERE referrer_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []*domain.Referral
	for rows.Next() {
		ref := &domain.Referral{}
		if err := rows.Scan(&ref.ID, &ref.ReferrerID, &ref.ReferredID, &ref.ReferralCode, &ref.Stage1Applied, &ref.Stage1BonusDays, &ref.Stage2Applied, &ref.Stage2BonusDays, &ref.GaveDiscount, &ref.CreatedAt, &ref.UpdatedAt); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (r *PostgresRepository) GetReferralStats(ctx context.Context, userID int64) (*domain.ReferralStats, error) {
	stats := &domain.ReferralStats{}

	query := `
	  SELECT COUNT(*), COUNT(*) FILTER (WHERE gave_discount = false), COUNT(*) FILTER (WHERE gave_discount = true),
			 COUNT(*) FILTER (WHERE stage1_applied = true), COUNT(*) FILTER (WHERE stage2_applied = true),
			 COALESCE(SUM(stage1_bonus_days + stage2_bonus_days), 0)
	  FROM referrals WHERE referrer_id = $1`

	r.db.QueryRow(ctx, query, userID).Scan(&stats.TotalReferrals, &stats.BonusReferrals, &stats.DiscountReferrals, &stats.Stage1Completed, &stats.Stage2Completed, &stats.TotalBonusDays)
	user, _ := r.GetUserByID(ctx, userID)
	if user != nil {
		stats.AccumulatedDiscount = user.DiscountPercent
	}

	streak, _ := r.GetUserOverallStreak(ctx, userID)
	stats.CurrentStreak = streak
	stats.CanInvite = streak >= domain.ReferralUnlockStreak
	if !stats.CanInvite {
		stats.DaysUntilUnlock = domain.ReferralUnlockStreak - streak
	}
	return stats, nil
}

func (r *PostgresRepository) UpdateReferralStage1(ctx context.Context, referralID int64, bonusDays int) error {
	_, err := r.db.Exec(ctx, `UPDATE referrals SET stage1_applied=true, stage1_bonus_days=$2, updated_at=$3 WHERE id=$1`, referralID, bonusDays, time.Now())
	return err
}

func (r *PostgresRepository) UpdateReferralStage2(ctx context.Context, referralID int64, bonusDays int) error {
	_, err := r.db.Exec(ctx, `UPDATE referrals SET stage2_applied=true, stage2_bonus_days=$2, updated_at=$3 WHERE id=$1`, referralID, bonusDays, time.Now())
	return err
}

func (r *PostgresRepository) UpdateReferralDiscount(ctx context.Context, referralID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE referrals SET gave_discount=true, updated_at=$2 WHERE id=$1`, referralID, time.Now())
	return err
}

func (r *PostgresRepository) GetPendingStage2Referrals(ctx context.Context, referredID int64) (*domain.Referral, error) {
	query := `
	  SELECT id, referrer_id, referred_id, referral_code, stage1_applied, stage1_bonus_days, stage2_applied, stage2_bonus_days, gave_discount, created_at, updated_at
	  FROM referrals WHERE referred_id = $1 AND stage1_applied = true AND stage2_applied = false AND gave_discount = false`

	ref := &domain.Referral{}
	err := r.db.QueryRow(ctx, query, referredID).Scan(&ref.ID, &ref.ReferrerID, &ref.ReferredID, &ref.ReferralCode, &ref.Stage1Applied, &ref.Stage1BonusDays, &ref.Stage2Applied, &ref.Stage2BonusDays, &ref.GaveDiscount, &ref.CreatedAt, &ref.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return ref, err
}

func (r *PostgresRepository) CountBonusReferrals(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM referrals WHERE referrer_id = $1 AND gave_discount = false`, userID).Scan(&count)
	return count, err
}

// ==================== ACHIEVEMENTS ====================

func (r *PostgresRepository) CreateAchievement(ctx context.Context, a *domain.Achievement) error {
	query := `INSERT INTO achievements (user_id, type, streak_days, bonus_days, unlocked_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (user_id, type) DO NOTHING RETURNING id`
	err := r.db.QueryRow(ctx, query, a.UserID, a.Type, a.StreakDays, a.BonusDays, time.Now()).Scan(&a.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	return err
}

func (r *PostgresRepository) GetUserAchievements(ctx context.Context, userID int64) ([]*domain.Achievement, error) {
	rows, err := r.db.Query(ctx, `SELECT id, user_id, type, streak_days, bonus_days, unlocked_at FROM achievements WHERE user_id = $1 ORDER BY unlocked_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var achievements []*domain.Achievement
	for rows.Next() {
		a := &domain.Achievement{}
		if err := rows.Scan(&a.ID, &a.UserID, &a.Type, &a.StreakDays, &a.BonusDays, &a.UnlockedAt); err != nil {
			return nil, err
		}
		achievements = append(achievements, a)
	}
	return achievements, nil
}

func (r *PostgresRepository) HasAchievement(ctx context.Context, userID int64, t domain.AchievementType) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM achievements WHERE user_id = $1 AND type = $2)`, userID, t).Scan(&exists)
	return exists, err
}

// ==================== ADS ====================
func (r *PostgresRepository) CreateAd(ctx context.Context, ad *domain.Ad) error {
	query := `INSERT INTO ads (name, text, image_url, button_text, button_url, is_active, priority, start_date, end_date, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) RETURNING id`
	return r.db.QueryRow(ctx, query, ad.Name, ad.Text, ad.ImageURL, ad.ButtonText, ad.ButtonURL, ad.IsActive, ad.Priority, ad.StartDate, ad.EndDate, time.Now()).Scan(&ad.ID)
}

func (r *PostgresRepository) GetActiveAds(ctx context.Context) ([]*domain.Ad, error) {
	query := `SELECT id, name, text, image_url, button_text, button_url, is_active, priority, views_count, clicks_count, start_date, end_date, created_at
	  FROM ads WHERE is_active = true AND (start_date IS NULL OR start_date <= NOW()) AND (end_date IS NULL OR end_date >= NOW()) ORDER BY priority DESC`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ads []*domain.Ad
	for rows.Next() {
		a := &domain.Ad{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Text, &a.ImageURL, &a.ButtonText, &a.ButtonURL, &a.IsActive, &a.Priority, &a.ViewsCount, &a.ClicksCount, &a.StartDate, &a.EndDate, &a.CreatedAt); err != nil {
			return nil, err
		}
		ads = append(ads, a)
	}
	return ads, nil
}

func (r *PostgresRepository) GetAdByID(ctx context.Context, id int64) (*domain.Ad, error) {
	query := `SELECT id, name, text, image_url, button_text, button_url, is_active, priority, views_count, clicks_count, start_date, end_date, created_at FROM ads WHERE id = $1`
	a := &domain.Ad{}
	err := r.db.QueryRow(ctx, query, id).Scan(&a.ID, &a.Name, &a.Text, &a.ImageURL, &a.ButtonText, &a.ButtonURL, &a.IsActive, &a.Priority, &a.ViewsCount, &a.ClicksCount, &a.StartDate, &a.EndDate, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

func (r *PostgresRepository) GetAllAds(ctx context.Context) ([]*domain.Ad, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, text, image_url, button_text, button_url, is_active, priority, views_count, clicks_count, start_date, end_date, created_at FROM ads ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ads []*domain.Ad
	for rows.Next() {
		a := &domain.Ad{}
		if err := rows.Scan(&a.ID, &a.Name, &a.Text, &a.ImageURL, &a.ButtonText, &a.ButtonURL, &a.IsActive, &a.Priority, &a.ViewsCount, &a.ClicksCount, &a.StartDate, &a.EndDate, &a.CreatedAt); err != nil {
			return nil, err
		}
		ads = append(ads, a)
	}
	return ads, nil
}

func (r *PostgresRepository) UpdateAd(ctx context.Context, ad *domain.Ad) error {
	query := `UPDATE ads SET name=$2, text=$3, image_url=$4, button_text=$5, button_url=$6, is_active=$7, priority=$8, start_date=$9, end_date=$10, updated_at=$11 WHERE id=$1`
	_, err := r.db.Exec(ctx, query, ad.ID, ad.Name, ad.Text, ad.ImageURL, ad.ButtonText, ad.ButtonURL, ad.IsActive, ad.Priority, ad.StartDate, ad.EndDate, time.Now())
	return err
}

func (r *PostgresRepository) DeleteAd(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM ads WHERE id = $1`, id)
	return err
}

func (r *PostgresRepository) IncrementAdViews(ctx context.Context, adID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE ads SET views_count = views_count + 1 WHERE id = $1`, adID)
	return err
}

func (r *PostgresRepository) IncrementAdClicks(ctx context.Context, adID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE ads SET clicks_count = clicks_count + 1 WHERE id = $1`, adID)
	return err
}

// ==================== BROADCASTS ====================

func (r *PostgresRepository) CreateBroadcast(ctx context.Context, b *domain.Broadcast) error {
	query := `INSERT INTO broadcasts (name, text, image_url, button_text, button_url, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`
	return r.db.QueryRow(ctx, query, b.Name, b.Text, b.ImageURL, b.ButtonText, b.ButtonURL, b.Status, time.Now()).Scan(&b.ID)
}
func (r *PostgresRepository) GetBroadcastByID(ctx context.Context, id int64) (*domain.Broadcast, error) {
	query := `SELECT id, name, text, image_url, button_text, button_url, status, total_users, sent_count, failed_count, last_user_id, created_at, started_at, completed_at FROM broadcasts WHERE id = $1`
	b := &domain.Broadcast{}
	err := r.db.QueryRow(ctx, query, id).Scan(&b.ID, &b.Name, &b.Text, &b.ImageURL, &b.ButtonText, &b.ButtonURL, &b.Status, &b.TotalUsers, &b.SentCount, &b.FailedCount, &b.LastUserID, &b.CreatedAt, &b.StartedAt, &b.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (r *PostgresRepository) GetAllBroadcasts(ctx context.Context) ([]*domain.Broadcast, error) {
	rows, err := r.db.Query(ctx, `SELECT id, name, text, image_url, button_text, button_url, status, total_users, sent_count, failed_count, last_user_id, created_at, started_at, completed_at FROM broadcasts ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var broadcasts []*domain.Broadcast
	for rows.Next() {
		b := &domain.Broadcast{}
		if err := rows.Scan(&b.ID, &b.Name, &b.Text, &b.ImageURL, &b.ButtonText, &b.ButtonURL, &b.Status, &b.TotalUsers, &b.SentCount, &b.FailedCount, &b.LastUserID, &b.CreatedAt, &b.StartedAt, &b.CompletedAt); err != nil {
			return nil, err
		}
		broadcasts = append(broadcasts, b)
	}
	return broadcasts, nil
}

func (r *PostgresRepository) GetRunningBroadcast(ctx context.Context) (*domain.Broadcast, error) {
	b := &domain.Broadcast{}
	err := r.db.QueryRow(ctx, `SELECT id, name, text, image_url, button_text, button_url, status, total_users, sent_count, failed_count, last_user_id, created_at, started_at, completed_at FROM broadcasts WHERE status = 'running' LIMIT 1`).Scan(&b.ID, &b.Name, &b.Text, &b.ImageURL, &b.ButtonText, &b.ButtonURL, &b.Status, &b.TotalUsers, &b.SentCount, &b.FailedCount, &b.LastUserID, &b.CreatedAt, &b.StartedAt, &b.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (r *PostgresRepository) UpdateBroadcastStatus(ctx context.Context, id int64, status domain.BroadcastStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE broadcasts SET status=$2 WHERE id=$1, id, status`)
	return err
}

func (r *PostgresRepository) UpdateBroadcastProgress(ctx context.Context, id int64, sent, failed int, lastUserID int64) error {
	_, err := r.db.Exec(ctx, `UPDATE broadcasts SET sent_count=$2, failed_count=$3, last_user_id=$4 WHERE id=$1`, id, sent, failed, lastUserID)
	return err
}

func (r *PostgresRepository) StartBroadcast(ctx context.Context, id int64, totalUsers int) error {
	_, err := r.db.Exec(ctx, `UPDATE broadcasts SET status='running', total_users=$2, started_at=$3 WHERE id=$1`, id, totalUsers, time.Now())
	return err
}

func (r *PostgresRepository) CompleteBroadcast(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `UPDATE broadcasts SET status='completed', completed_at=$2 WHERE id=$1`, id, time.Now())
	return err
}

// ==================== ADMINS ====================

func (r *PostgresRepository) IsAdmin(ctx context.Context, telegramID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admins WHERE telegram_id = $1)`, telegramID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) AddAdmin(ctx context.Context, telegramID int64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO admins (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING`, telegramID)
	return err
}

// ==================== EXPORT ====================

func (r *PostgresRepository) GetAllUserData(ctx context.Context, userID int64) (*UserExportData, error) {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	habits, _ := r.GetActiveHabits(ctx, userID)
	logs, _ := r.GetUserLogsForPeriod(ctx, userID, time.Now().AddDate(-1, 0, 0), time.Now())
	achievements, _ := r.GetUserAchievements(ctx, userID)
	stats, _ := r.GetUserStats(ctx, userID)
	return &UserExportData{User: user, Habits: habits, Logs: logs, Achievements: achievements, Stats: stats}, nil
}

// ===== PROMOCODES =====

func (r *PostgresRepository) CreatePromocode(ctx context.Context, code string, discount int, maxUses int) error {
	var maxUsesPtr *int
	if maxUses > 0 {
		maxUsesPtr = &maxUses
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO promocodes (code, discount_percent, max_uses) VALUES ($1, $2, $3)`,
		code, discount, maxUsesPtr)
	return err
}

func (r *PostgresRepository) GetAllPromocodes(ctx context.Context) ([]*domain.Promocode, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, code, discount_percent, max_uses, used_count, is_active, created_at 
         FROM promocodes ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var promos []*domain.Promocode
	for rows.Next() {
		p := &domain.Promocode{}
		err := rows.Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.MaxUses, &p.UsedCount, &p.IsActive, &p.CreatedAt)
		if err != nil {
			return nil, err
		}
		promos = append(promos, p)
	}
	return promos, nil
}

func (r *PostgresRepository) GetPromocodeByCode(ctx context.Context, code string) (*domain.Promocode, error) {
	p := &domain.Promocode{}
	err := r.db.QueryRow(ctx,
		`SELECT id, code, discount_percent, max_uses, used_count, is_active, created_at 
         FROM promocodes WHERE code = $1`, code).
		Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.MaxUses, &p.UsedCount, &p.IsActive, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PostgresRepository) DeletePromocode(ctx context.Context, code string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM promocodes WHERE code = $1`, code)
	return err
}

func (r *PostgresRepository) TogglePromocode(ctx context.Context, code string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE promocodes SET is_active = NOT is_active WHERE code = $1`, code)
	return err
}

func (r *PostgresRepository) HasUserUsedPromocode(ctx context.Context, userID int64, promocodeID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM promocode_usages WHERE user_id = $1 AND promocode_id = $2)`,
		userID, promocodeID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) SetUserActivePromocode(ctx context.Context, userID int64, promocodeID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET active_promocode_id = $1 WHERE telegram_id = $2`,
		promocodeID, userID)
	return err
}

func (r *PostgresRepository) GetUserActivePromocode(ctx context.Context, userID int64) (*domain.Promocode, error) {
	p := &domain.Promocode{}
	err := r.db.QueryRow(ctx,
		`SELECT p.id, p.code, p.discount_percent, p.max_uses, p.used_count, p.is_active, p.created_at
         FROM promocodes p
         JOIN users u ON u.active_promocode_id = p.id
         WHERE u.telegram_id = $1 AND p.is_active = true`, userID).
		Scan(&p.ID, &p.Code, &p.DiscountPercent, &p.MaxUses, &p.UsedCount, &p.IsActive, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PostgresRepository) ClearUserActivePromocode(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE users SET active_promocode_id = NULL WHERE telegram_id = $1`, userID)
	return err
}

func (r *PostgresRepository) IncrementPromocodeUsage(ctx context.Context, promocodeID int64, userID int64) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO promocode_usages (promocode_id, user_id) VALUES ($1, $2)`,
		promocodeID, userID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx,
		`UPDATE promocodes SET used_count = used_count + 1 WHERE id = $1`,
		promocodeID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) GetUsersForFirstPromo(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Query(ctx, `
        SELECT u.telegram_id 
        FROM users u
        LEFT JOIN user_promo_status ups ON u.telegram_id = ups.user_id
        WHERE u.created_at <= NOW() - INTERVAL '1 day'
          AND (ups.first_promo_sent IS NULL OR ups.first_promo_sent = false)
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *PostgresRepository) MarkFirstPromoSent(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO user_promo_status (user_id, first_promo_sent)
        VALUES ($1, true)
        ON CONFLICT (user_id) DO UPDATE SET first_promo_sent = true
    `, userID)
	return err
}

func (r *PostgresRepository) GetUsersForWeeklyPromo(ctx context.Context) ([]int64, error) {
	rows, err := r.db.Query(ctx, `
        SELECT u.telegram_id 
        FROM users u
        LEFT JOIN user_promo_status ups ON u.telegram_id = ups.user_id
        WHERE ups.first_promo_sent = true
          AND (ups.last_weekly_promo IS NULL OR ups.last_weekly_promo < NOW() - INTERVAL '7 days')
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *PostgresRepository) MarkWeeklyPromoSent(ctx context.Context, userID int64) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO user_promo_status (user_id, last_weekly_promo)
        VALUES ($1, NOW())
        ON CONFLICT (user_id) DO UPDATE SET last_weekly_promo = NOW()
    `, userID)
	return err
}

func (r *PostgresRepository) UpdateHabitReminder(ctx context.Context, habitID int64, reminderTime *string, reminderDays []int) error {
	_, err := r.db.Exec(ctx, `
        UPDATE habits 
        SET reminder_time = $1, reminder_days = $2
        WHERE id = $3
    `, reminderTime, reminderDays, habitID)
	return err
}

// ===== CHARTS =====

// GetWeeklyCompletionStats — количество выполненных привычек по дням за 7 дней
func (r *PostgresRepository) GetWeeklyCompletionStats(ctx context.Context, userID int64) (map[string]int, error) {
	rows, err := r.db.Query(ctx, `
	  SELECT DATE(hc.completed_at) as date, COUNT(*) as count
	  FROM habit_completions hc
	  JOIN habits h ON hc.habit_id = h.id
	  WHERE h.user_id = $1 
		AND hc.completed_at >= CURRENT_DATE - INTERVAL '6 days'
	  GROUP BY DATE(hc.completed_at)
	  ORDER BY date
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var date time.Time
		var count int
		if err := rows.Scan(&date, &count); err != nil {
			return nil, err
		}
		result[date.Format("2006-01-02")] = count
	}
	return result, nil
}

// GetHabitCompletionDays — дни выполнения конкретной привычки
func (r *PostgresRepository) GetHabitCompletionDays(ctx context.Context, habitID int64, days int) (map[string]bool, error) {
	rows, err := r.db.Query(ctx, `
	  SELECT DATE(completed_at)
	  FROM habit_completions
	  WHERE habit_id = $1 
		AND completed_at >= CURRENT_DATE - $2 * INTERVAL '1 day'
	`, habitID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		result[date.Format("2006-01-02")] = true
	}
	return result, nil
}

// GetHabitsStreaks — серии всех привычек пользователя
func (r *PostgresRepository) GetHabitsStreaks(ctx context.Context, userID int64) ([]HabitStreak, error) {
	rows, err := r.db.Query(ctx, `
	  SELECT h.id, h.name, COALESCE(
		(SELECT COUNT(*)::int FROM (
		  SELECT DATE(completed_at) as d
		  FROM habit_completions
		  WHERE habit_id = h.id
			AND completed_at >= CURRENT_DATE - INTERVAL '365 days'
		  ORDER BY d DESC
		) dates
		WHERE d >= CURRENT_DATE - (ROW_NUMBER() OVER (ORDER BY d DESC) - 1) * INTERVAL '1 day'
		), 0
	  ) as streak
	  FROM habits h
	  WHERE h.user_id = $1 AND h.is_active = true
	  ORDER BY h.name
	`, userID)
	if err != nil {
		// Упрощённый запрос если сложный не работает
		return r.getHabitsStreaksSimple(ctx, userID)
	}
	defer rows.Close()

	var result []HabitStreak
	for rows.Next() {
		var hs HabitStreak
		if err := rows.Scan(&hs.HabitID, &hs.Name, &hs.Streak); err != nil {
			return nil, err
		}
		result = append(result, hs)
	}
	return result, nil
}

// Упрощённая версия
func (r *PostgresRepository) getHabitsStreaksSimple(ctx context.Context, userID int64) ([]HabitStreak, error) {
	rows, err := r.db.Query(ctx, `
	  SELECT h.id, h.name
	  FROM habits h
	  WHERE h.user_id = $1 AND h.is_active = true
	  ORDER BY h.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []HabitStreak
	for rows.Next() {
		var hs HabitStreak
		if err := rows.Scan(&hs.HabitID, &hs.Name); err != nil {
			return nil, err
		}
		// Получаем серию отдельно
		hs.Streak = r.calculateStreak(ctx, hs.HabitID)
		result = append(result, hs)
	}
	return result, nil
}

func (r *PostgresRepository) calculateStreak(ctx context.Context, habitID int64) int {
	rows, err := r.db.Query(ctx, `
	  SELECT DATE(completed_at) as d
	  FROM habit_completions
	  WHERE habit_id = $1
	  ORDER BY d DESC
	  LIMIT 365
	`, habitID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	var dates []time.Time
	for rows.Next() {
		var d time.Time
		rows.Scan(&d)
		dates = append(dates, d)
	}

	if len(dates) == 0 {
		return 0
	}

	streak := 0
	today := time.Now().Truncate(24 * time.Hour)

	for i, d := range dates {
		expected := today.AddDate(0, 0, -i)
		if d.Format("2006-01-02") == expected.Format("2006-01-02") {
			streak++
		} else {
			break
		}
	}

	return streak
}
