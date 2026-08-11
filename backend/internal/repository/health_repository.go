package repository

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthRepository interface {
	CheckPostgres(ctx context.Context) error
	CheckRedis(ctx context.Context) error
}

type healthRepository struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthRepository(db *gorm.DB, redisClient *redis.Client) HealthRepository {
	return &healthRepository{db: db, redis: redisClient}
}

func (r *healthRepository) CheckPostgres(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("get postgres pool: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

func (r *healthRepository) CheckRedis(ctx context.Context) error {
	if err := r.redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}
