package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/api"
	"github.com/NeoStackLab/NexaFlow/backend/internal/handler"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/cache"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/database"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/logger"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"go.uber.org/zap"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to the YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	log, err := logger.New(cfg.Log)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	installRepository := repository.NewInstallRepository(cfg)
	installService := service.NewInstallService(installRepository, version, cfg)
	installHandler := handler.NewInstallHandler(installService)
	authRepository := repository.NewAuthRepository(installRepository)
	authService := service.NewAuthService(authRepository, cfg.Auth)
	authHandler := handler.NewAuthHandler(authService)
	dynamicModelRepository := repository.NewDynamicModelRepository(installRepository)
	dynamicModelService := service.NewDynamicModelService(dynamicModelRepository)
	dynamicModelHandler := handler.NewDynamicModelHandler(dynamicModelService)
	billingRepository := repository.NewBillingRepository(installRepository)
	billingProvider := service.NewBillingProvider(cfg.Billing)
	billingService := service.NewBillingService(billingRepository, billingProvider)
	billingHandler := handler.NewBillingHandler(billingService)
	objectStore, err := service.NewObjectStore(startupContext, cfg.Storage)
	if err != nil {
		return fmt.Errorf("initialize object storage: %w", err)
	}
	fileRepository := repository.NewFileRepository(installRepository)
	fileService := service.NewFileService(fileRepository, objectStore)
	fileHandler := handler.NewFileHandler(fileService)
	recordRepository := repository.NewDynamicRecordRepository(installRepository)
	recordService := service.NewDynamicRecordService(recordRepository, dynamicModelService, billingService)
	recordHandler := handler.NewDynamicRecordHandler(recordService)
	formRepository := repository.NewFormRepository(installRepository)
	formService := service.NewFormService(formRepository, dynamicModelService)
	formHandler := handler.NewFormHandler(formService)
	dashboardRepository := repository.NewDashboardRepository(installRepository)
	dashboardService := service.NewDashboardService(dashboardRepository, dynamicModelService)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)
	workflowRepository := repository.NewWorkflowRepository(installRepository)
	workflowService := service.NewWorkflowService(workflowRepository, dynamicModelService, recordService)
	workflowHandler := handler.NewWorkflowHandler(workflowService)
	aiProvider := service.NewAIProvider(cfg.AI)
	knowledgeRepository := repository.NewKnowledgeRepository(installRepository)
	knowledgeService := service.NewKnowledgeService(knowledgeRepository, aiProvider, billingService)
	agentRepository := repository.NewAgentRepository(installRepository)
	agentService := service.NewAgentService(agentRepository, knowledgeService, aiProvider, dynamicModelService, recordService, billingService)
	aiHandler := handler.NewAIHandler(knowledgeService, agentService)
	healthRepository := repository.NewSetupHealthRepository(installRepository)

	if installService.Status().Installed {
		if err := installRepository.MigrateInstalled(startupContext); err != nil {
			return fmt.Errorf("migrate installed database: %w", err)
		}
		if err := billingRepository.ConfigurePrices(startupContext, cfg.Billing.StripeProPriceID, cfg.Billing.StripeEnterprisePriceID); err != nil {
			return fmt.Errorf("configure billing prices: %w", err)
		}
		runtimeConfig, err := installRepository.RuntimeConfig()
		if err != nil {
			return fmt.Errorf("load installation configuration: %w", err)
		}
		cfg.Database.Host = runtimeConfig.Database.Host
		cfg.Database.Port = runtimeConfig.Database.Port
		cfg.Database.Name = runtimeConfig.Database.Name
		cfg.Database.User = runtimeConfig.Database.User
		cfg.Database.Password = runtimeConfig.Database.Password
		cfg.Database.SSLMode = runtimeConfig.Database.SSLMode
		cfg.Redis.Host = runtimeConfig.Redis.Host
		cfg.Redis.Port = runtimeConfig.Redis.Port
		cfg.Redis.Password = runtimeConfig.Redis.Password
		cfg.Redis.Database = runtimeConfig.Redis.Database

		db, err := database.Open(startupContext, cfg.Database)
		if err != nil {
			return err
		}
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("get postgres connection pool: %w", err)
		}
		defer func() { _ = sqlDB.Close() }()
		redisClient, err := cache.Open(startupContext, cfg.Redis)
		if err != nil {
			return err
		}
		defer func() { _ = redisClient.Close() }()
		healthRepository = repository.NewHealthRepository(db, redisClient)
	} else {
		log.Info("installation_required", zap.String("path", cfg.Install.DataDir))
	}

	healthService := service.NewHealthService(healthRepository, cfg.App.Name, version)
	healthHandler := handler.NewHealthHandler(healthService)
	router := api.NewRouter(cfg, log, healthHandler, installHandler, authHandler, dynamicModelHandler, recordHandler, formHandler, workflowHandler, aiHandler, billingHandler, fileHandler, dashboardHandler, authService)

	server := &http.Server{
		Addr:         net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("server_started", zap.String("address", server.Addr), zap.String("version", version))
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}
	case <-shutdownSignals.Done():
		log.Info("server_shutdown_started")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	log.Info("server_stopped")
	return nil
}
