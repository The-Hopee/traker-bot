CREATE TABLE IF NOT EXISTS user_promo_status (
    user_id BIGINT PRIMARY KEY REFERENCES users(telegram_id),
    first_promo_sent BOOLEAN DEFAULT false,
    last_weekly_promo TIMESTAMP DEFAULT NULL
);