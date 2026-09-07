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
	"github.com/NeoStackLab/NexaFlow/backend/internal/jobs"
	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/observability"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/cache"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/logger"
	"github.com/NeoStackLab/NexaFlow/backend/internal/repository"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/NeoStackLab/NexaFlow/backend/internal/tenantcontext"
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
	telemetry, err := observability.Start(startupContext, cfg.Observability, cfg.App.Name, version, cfg.App.Env)
	if err != nil {
		return fmt.Errorf("initialize observability: %w", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := telemetry.Shutdown(shutdownContext); shutdownErr != nil {
			log.Warn("telemetry_shutdown_failed", zap.Error(shutdownErr))
		}
	}()

	installRepository := repository.NewInstallRepository(cfg)
	installService := service.NewInstallService(installRepository, version, cfg)
	installHandler := handler.NewInstallHandler(installService)
	auditRepository := repository.NewAuditRepository(installRepository)
	auditService := service.NewAuditService(auditRepository)
	auditHandler := handler.NewAuditHandler(auditService)
	authRepository := repository.NewAuthRepository(installRepository)
	dynamicModelRepository := repository.NewDynamicModelRepository(installRepository)
	dynamicModelService := service.NewDynamicModelService(dynamicModelRepository)
	dynamicModelHandler := handler.NewDynamicModelHandler(dynamicModelService)
	billingRepository := repository.NewBillingRepository(installRepository)
	billingProvider := service.NewBillingProvider(cfg.Billing)
	billingService := service.NewBillingService(billingRepository, billingProvider)
	billingHandler := handler.NewBillingHandler(billingService)
	authService := service.NewAuthService(authRepository, cfg.Auth, billingService)
	authHandler := handler.NewAuthHandler(authService)
	tenantRepository := repository.NewTenantRepository(installRepository)
	tenantService := service.NewTenantService(tenantRepository)
	tenantHandler := handler.NewTenantHandler(tenantService)
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
	formSubmissionRepository := repository.NewFormSubmissionRepository(installRepository)
	formSubmissionService := service.NewFormSubmissionService(formService, recordService, formSubmissionRepository)
	formHandler := handler.NewFormHandler(formService, formSubmissionService)
	dashboardRepository := repository.NewDashboardRepository(installRepository)
	dashboardService := service.NewDashboardService(dashboardRepository, dynamicModelService)
	dashboardHandler := handler.NewDashboardHandler(dashboardService)
	workflowRepository := repository.NewWorkflowRepository(installRepository)
	workflowService := service.NewWorkflowService(workflowRepository, dynamicModelService, recordService)
	workflowHandler := handler.NewWorkflowHandler(workflowService)
	notificationRepository := repository.NewNotificationRepository(installRepository)
	aiProvider := service.NewAIProvider(cfg.AI)
	knowledgeRepository := repository.NewKnowledgeRepository(installRepository)
	knowledgeService := service.NewKnowledgeService(knowledgeRepository, aiProvider, billingService)
	knowledgeIngestionRepository := repository.NewKnowledgeIngestionRepository(installRepository)
	knowledgeIngestionService := service.NewKnowledgeIngestionService(knowledgeIngestionRepository, objectStore)
	agentRepository := repository.NewAgentRepository(installRepository)
	agentService := service.NewAgentService(agentRepository, knowledgeService, aiProvider, dynamicModelService, recordService, billingService)
	aiHandler := handler.NewAIHandler(knowledgeService, agentService, knowledgeIngestionService)
	healthRepository := repository.NewSetupHealthRepository(installRepository)
	authRateLimit := appmiddleware.RateLimit(60, time.Minute)
	var jobQueue *jobs.Queue

	installed := installService.Status().Installed
	if installed {
		if err := installRepository.MigrateInstalled(startupContext); err != nil {
			return fmt.Errorf("migrate installed database: %w", err)
		}
		if err := billingRepository.ConfigurePrices(tenantcontext.WithSystem(startupContext), cfg.Billing.StripeProPriceID, cfg.Billing.StripeEnterprisePriceID); err != nil {
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

		runtimeDBProvider, ok := installRepository.(repository.RuntimeDBProvider)
		if !ok {
			return errors.New("install repository does not provide a shared runtime database")
		}
		db, err := runtimeDBProvider.RuntimeDB(startupContext)
		if err != nil {
			return fmt.Errorf("open shared runtime database: %w", err)
		}
		defer func() { _ = runtimeDBProvider.CloseRuntimeDB() }()
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("access database pool: %w", err)
		}
		if err := observability.InstrumentDatabase(db, sqlDB); err != nil {
			return err
		}
		redisClient, err := cache.Open(startupContext, cfg.Redis)
		if err != nil {
			return err
		}
		defer func() { _ = redisClient.Close() }()
		if err := observability.InstrumentRedis(redisClient); err != nil {
			return fmt.Errorf("instrument Redis: %w", err)
		}
		authRateLimit = appmiddleware.RedisRateLimit(redisClient, cfg.Jobs.Namespace, 60, time.Minute)
		jobQueue = jobs.New(redisClient, cfg.Jobs, log, telemetry.Metrics)
		notificationWorker := service.NewNotificationWorker(notificationRepository, cfg.Notifications, log)
		knowledgeWorker := service.NewKnowledgeIngestionWorker(knowledgeIngestionRepository, objectStore, knowledgeService, cfg.Knowledge, log)
		if err := jobQueue.Register("notifications.dispatch", func(ctx context.Context, _ jobs.Job) error { return notificationWorker.ProcessBatch(ctx) }); err != nil {
			return err
		}
		if err := jobQueue.Register("knowledge.ingest", func(ctx context.Context, _ jobs.Job) error { return knowledgeWorker.ProcessBatch(ctx) }); err != nil {
			return err
		}
		if err := jobQueue.Schedule("notifications.dispatch", cfg.Notifications.WorkerInterval, jobs.Request{Kind: "notifications.dispatch", Priority: 7, MaxAttempts: cfg.Notifications.MaxAttempts}); err != nil {
			return err
		}
		if err := jobQueue.Schedule("knowledge.ingest", cfg.Knowledge.WorkerInterval, jobs.Request{Kind: "knowledge.ingest", Priority: 6, MaxAttempts: cfg.Knowledge.MaxAttempts}); err != nil {
			return err
		}
		healthRepository = repository.NewHealthRepository(db, redisClient)
	} else {
		log.Info("installation_required", zap.String("path", cfg.Install.DataDir))
	}

	healthService := service.NewHealthService(healthRepository, cfg.App.Name, version)
	healthHandler := handler.NewHealthHandler(healthService)
	router := api.NewRouter(cfg, log, healthHandler, installHandler, authHandler, tenantHandler, dynamicModelHandler, recordHandler, formHandler, workflowHandler, aiHandler, billingHandler, fileHandler, dashboardHandler, auditHandler, authService, auditService, telemetry.Metrics.HTTPMiddleware(), authRateLimit)

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
	var jobRuntimeDone chan error
	if installed && jobQueue != nil {
		jobRuntimeDone = make(chan error, 1)
		go func() {
			err := jobQueue.Run(shutdownSignals)
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Error("job_runtime_stopped", zap.Error(err))
			}
			jobRuntimeDone <- err
		}()
	}

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve http: %w", err)
		}
	case <-shutdownSignals.Done():
		log.Info("server_shutdown_started")
	}
	stop()

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	if jobRuntimeDone != nil {
		select {
		case <-jobRuntimeDone:
		case <-shutdownContext.Done():
			return fmt.Errorf("shutdown job runtime: %w", shutdownContext.Err())
		}
	}

	log.Info("server_stopped")
	return nil
}
