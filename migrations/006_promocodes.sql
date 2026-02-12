CREATE TABLE IF NOT EXISTS promocodes (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) UNIQUE NOT NULL,
    discount_percent INT NOT NULL DEFAULT 0,
    max_uses INT DEFAULT NULL,
    used_count INT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS promocode_usages (
    id SERIAL PRIMARY KEY,
    promocode_id INT REFERENCES promocodes(id),
    user_id BIGINT REFERENCES users(telegram_id),
    used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(promocode_id, user_id)
);

-- Активный промокод юзера (который он ввёл, но ещё не оплатил)
ALTER TABLE users ADD COLUMN IF NOT EXISTS active_promocode_id INT REFERENCES promocodes(id);