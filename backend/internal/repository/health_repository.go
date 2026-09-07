package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/cache"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
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

type setupHealthRepository struct{ install InstallRepository }

func NewSetupHealthRepository(install InstallRepository) HealthRepository {
	return setupHealthRepository{install: install}
}

func (r setupHealthRepository) CheckPostgres(ctx context.Context) error {
	runtimeConfig, err := r.install.RuntimeConfig()
	if err != nil {
		return errors.New("installation required")
	}
	db, err := database.Open(ctx, config.DatabaseConfig{
		Host: runtimeConfig.Database.Host, Port: runtimeConfig.Database.Port, User: runtimeConfig.Database.User,
		Password: runtimeConfig.Database.Password, Name: runtimeConfig.Database.Name, SSLMode: runtimeConfig.Database.SSLMode,
		MaxOpenConnections: 2, MaxIdleConnections: 1,
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (r setupHealthRepository) CheckRedis(ctx context.Context) error {
	runtimeConfig, err := r.install.RuntimeConfig()
	if err != nil {
		return errors.New("installation required")
	}
	client, err := cache.Open(ctx, config.RedisConfig{
		Host: runtimeConfig.Redis.Host, Port: runtimeConfig.Redis.Port, Password: runtimeConfig.Redis.Password,
		Database: runtimeConfig.Redis.Database,
	})
	if err != nil {
		return err
	}
	return client.Close()
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
