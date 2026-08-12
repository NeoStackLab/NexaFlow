package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app" validate:"required"`
	Install  InstallConfig  `mapstructure:"install" validate:"required"`
	Auth     AuthConfig     `mapstructure:"auth" validate:"required"`
	Server   ServerConfig   `mapstructure:"server" validate:"required"`
	Database DatabaseConfig `mapstructure:"database" validate:"required"`
	Redis    RedisConfig    `mapstructure:"redis" validate:"required"`
	CORS     CORSConfig     `mapstructure:"cors" validate:"required"`
	Log      LogConfig      `mapstructure:"log" validate:"required"`
	AI       AIConfig       `mapstructure:"ai" validate:"required"`
	Billing  BillingConfig  `mapstructure:"billing" validate:"required"`
	Storage  StorageConfig  `mapstructure:"storage" validate:"required"`
}

type StorageConfig struct {
	Provider        string `mapstructure:"provider" validate:"required,oneof=local s3 r2"`
	LocalPath       string `mapstructure:"local_path" validate:"required_if=Provider local"`
	Endpoint        string `mapstructure:"endpoint"`
	Region          string `mapstructure:"region"`
	Bucket          string `mapstructure:"bucket"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
}

type BillingConfig struct {
	StripeSecretKey         string `mapstructure:"stripe_secret_key"`
	StripeWebhookSecret     string `mapstructure:"stripe_webhook_secret"`
	StripeProPriceID        string `mapstructure:"stripe_pro_price_id"`
	StripeEnterprisePriceID string `mapstructure:"stripe_enterprise_price_id"`
	SuccessURL              string `mapstructure:"success_url" validate:"required,url"`
	CancelURL               string `mapstructure:"cancel_url" validate:"required,url"`
}

type AIConfig struct {
	BaseURL        string        `mapstructure:"base_url" validate:"required,url"`
	APIKey         string        `mapstructure:"api_key"`
	ChatModel      string        `mapstructure:"chat_model" validate:"required"`
	EmbeddingModel string        `mapstructure:"embedding_model" validate:"required"`
	Timeout        time.Duration `mapstructure:"timeout" validate:"required"`
}

type InstallConfig struct {
	DataDir string `mapstructure:"data_dir" validate:"required"`
	Mode    string `mapstructure:"mode" validate:"required,oneof=docker manual"`
}

type AuthConfig struct {
	JWTSecret       string        `mapstructure:"jwt_secret" validate:"required,min=32"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl" validate:"required"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl" validate:"required"`
	Issuer          string        `mapstructure:"issuer" validate:"required"`
}

type AppConfig struct {
	Name string `mapstructure:"name" validate:"required"`
	Env  string `mapstructure:"env" validate:"required,oneof=development test staging production"`
}

type ServerConfig struct {
	Host            string        `mapstructure:"host" validate:"required"`
	Port            int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout" validate:"required"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout" validate:"required"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout" validate:"required"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout" validate:"required"`
}

type DatabaseConfig struct {
	Host                  string        `mapstructure:"host" validate:"required"`
	Port                  int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	User                  string        `mapstructure:"user" validate:"required"`
	Password              string        `mapstructure:"password" validate:"required"`
	Name                  string        `mapstructure:"name" validate:"required"`
	SSLMode               string        `mapstructure:"sslmode" validate:"required,oneof=disable allow prefer require verify-ca verify-full"`
	MaxOpenConnections    int           `mapstructure:"max_open_connections" validate:"required,min=1"`
	MaxIdleConnections    int           `mapstructure:"max_idle_connections" validate:"required,min=1"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connection_max_lifetime" validate:"required"`
}

type RedisConfig struct {
	Host         string        `mapstructure:"host" validate:"required"`
	Port         int           `mapstructure:"port" validate:"required,min=1,max=65535"`
	Password     string        `mapstructure:"password"`
	Database     int           `mapstructure:"database" validate:"min=0"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" validate:"required"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" validate:"required"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" validate:"required"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins" validate:"required,min=1,dive,url"`
}

type LogConfig struct {
	Level  string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
	Format string `mapstructure:"format" validate:"required,oneof=console json"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	if cfg.App.Env == "production" && cfg.Auth.JWTSecret == "change-this-development-secret-now" {
		return nil, fmt.Errorf("validate config: AUTH_JWT_SECRET must be changed in production")
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "NexaFlow API")
	v.SetDefault("app.env", "development")
	v.SetDefault("install.data_dir", "./data")
	v.SetDefault("install.mode", "docker")
	v.SetDefault("auth.jwt_secret", "change-this-development-secret-now")
	v.SetDefault("auth.access_token_ttl", "15m")
	v.SetDefault("auth.refresh_token_ttl", "168h")
	v.SetDefault("auth.issuer", "nexaflow")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "10s")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "nexaflow")
	v.SetDefault("database.password", "nexaflow_dev")
	v.SetDefault("database.name", "nexaflow")
	v.SetDefault("database.sslmode", "disable")
	v.SetDefault("database.max_open_connections", 25)
	v.SetDefault("database.max_idle_connections", 5)
	v.SetDefault("database.connection_max_lifetime", "30m")
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.password", "")
	v.SetDefault("redis.database", 0)
	v.SetDefault("redis.dial_timeout", "5s")
	v.SetDefault("redis.read_timeout", "3s")
	v.SetDefault("redis.write_timeout", "3s")
	v.SetDefault("cors.allowed_origins", []string{"http://localhost:3000"})
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("ai.base_url", "https://api.openai.com/v1")
	v.SetDefault("ai.api_key", "")
	v.SetDefault("ai.chat_model", "gpt-5-mini")
	v.SetDefault("ai.embedding_model", "text-embedding-3-small")
	v.SetDefault("ai.timeout", "60s")
	v.SetDefault("billing.stripe_secret_key", "")
	v.SetDefault("billing.stripe_webhook_secret", "")
	v.SetDefault("billing.stripe_pro_price_id", "")
	v.SetDefault("billing.stripe_enterprise_price_id", "")
	v.SetDefault("billing.success_url", "http://localhost:3000/admin/billing?checkout=success")
	v.SetDefault("billing.cancel_url", "http://localhost:3000/admin/billing?checkout=cancel")
	v.SetDefault("storage.provider", "local")
	v.SetDefault("storage.local_path", "./data/uploads")
	v.SetDefault("storage.endpoint", "")
	v.SetDefault("storage.region", "auto")
	v.SetDefault("storage.bucket", "")
	v.SetDefault("storage.access_key_id", "")
	v.SetDefault("storage.secret_access_key", "")
}
