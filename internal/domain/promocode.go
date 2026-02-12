package domain

import "time"

type Promocode struct {
	ID              int64
	Code            string
	DiscountPercent int
	MaxUses         *int
	UsedCount       int
	IsActive        bool
	CreatedAt       time.Time
}
