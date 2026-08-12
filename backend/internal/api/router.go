package api

import (
	"net/http"

	"github.com/NeoStackLab/NexaFlow/backend/internal/handler"
	appmiddleware "github.com/NeoStackLab/NexaFlow/backend/internal/middleware"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/httpx"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewRouter(cfg *config.Config, log *zap.Logger, healthHandler *handler.HealthHandler, installHandler *handler.InstallHandler, authHandler *handler.AuthHandler, dynamicModelHandler *handler.DynamicModelHandler, recordHandler *handler.DynamicRecordHandler, formHandler *handler.FormHandler, workflowHandler *handler.WorkflowHandler, aiHandler *handler.AIHandler, billingHandler *handler.BillingHandler, fileHandler *handler.FileHandler, dashboardHandler *handler.DashboardHandler, authService service.AuthService) *gin.Engine {
	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(
		appmiddleware.RequestID(),
		appmiddleware.RequestLogger(log),
		appmiddleware.Recovery(log),
		appmiddleware.CORS(cfg.CORS.AllowedOrigins),
	)

	router.GET("/health/live", healthHandler.Liveness)
	router.GET("/health/ready", healthHandler.Readiness)

	v1 := router.Group("/api/v1")
	v1.GET("/health", healthHandler.Readiness)
	install := v1.Group("/install")
	install.GET("/status", installHandler.Status)
	install.GET("/environment", installHandler.Environment)
	install.GET("/readiness", installHandler.Readiness)
	install.POST("/complete", installHandler.Complete)
	auth := v1.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/refresh", authHandler.Refresh)
	auth.POST("/switch-tenant", authHandler.SwitchTenant)
	auth.POST("/logout", authHandler.Logout)
	protected := auth.Group("")
	protected.Use(appmiddleware.Authenticate(authService))
	protected.GET("/me", authHandler.Me)
	protected.GET("/sessions", authHandler.Sessions)
	protected.DELETE("/sessions/:sessionID", authHandler.RevokeSession)
	protected.GET("/menu", authHandler.Menu)
	protected.GET("/tenants", authHandler.Tenants)
	protected.POST("/tenants", authHandler.CreateTenant)
	protected.GET("/roles", appmiddleware.RequirePermission(authService, "role.manage"), authHandler.Roles)
	protected.GET("/permissions", appmiddleware.RequirePermission(authService, "role.manage"), authHandler.Permissions)
	protected.PUT("/roles/:roleID/permissions", appmiddleware.RequirePermission(authService, "role.manage"), authHandler.SetRolePermissions)
	protected.GET("/users", appmiddleware.RequirePermission(authService, "user.view"), authHandler.Users)
	protected.PUT("/users/:userID/roles", appmiddleware.RequirePermission(authService, "role.manage"), authHandler.SetUserRoles)
	entities := v1.Group("/entities")
	entities.Use(appmiddleware.Authenticate(authService))
	entities.GET("", appmiddleware.RequirePermission(authService, "entity.view"), dynamicModelHandler.List)
	entities.POST("", appmiddleware.RequirePermission(authService, "entity.manage"), dynamicModelHandler.Define)
	entities.GET("/:entityID", appmiddleware.RequirePermission(authService, "entity.view"), dynamicModelHandler.Get)
	entities.PUT("/:entityID", appmiddleware.RequirePermission(authService, "entity.manage"), dynamicModelHandler.Define)
	entities.DELETE("/:entityID", appmiddleware.RequirePermission(authService, "entity.manage"), dynamicModelHandler.Archive)
	records := v1.Group("/entities/:entityID/records")
	records.Use(appmiddleware.Authenticate(authService))
	records.GET("", appmiddleware.RequirePermission(authService, "record.view"), recordHandler.List)
	records.POST("", appmiddleware.RequirePermission(authService, "record.manage"), recordHandler.Create)
	records.GET("/:recordID", appmiddleware.RequirePermission(authService, "record.view"), recordHandler.Get)
	records.PUT("/:recordID", appmiddleware.RequirePermission(authService, "record.manage"), recordHandler.Update)
	records.DELETE("/:recordID", appmiddleware.RequirePermission(authService, "record.manage"), recordHandler.Delete)
	forms := v1.Group("/forms")
	forms.Use(appmiddleware.Authenticate(authService))
	forms.GET("", appmiddleware.RequirePermission(authService, "form.view"), formHandler.List)
	forms.POST("", appmiddleware.RequirePermission(authService, "form.manage"), formHandler.Define)
	forms.GET("/:formID", appmiddleware.RequirePermission(authService, "form.view"), formHandler.Get)
	forms.PUT("/:formID", appmiddleware.RequirePermission(authService, "form.manage"), formHandler.Define)
	forms.DELETE("/:formID", appmiddleware.RequirePermission(authService, "form.manage"), formHandler.Archive)
	workflows := v1.Group("/workflows")
	workflows.Use(appmiddleware.Authenticate(authService))
	workflows.GET("", appmiddleware.RequirePermission(authService, "workflow.view"), workflowHandler.List)
	workflows.POST("", appmiddleware.RequirePermission(authService, "workflow.manage"), workflowHandler.Define)
	workflows.GET("/:workflowID", appmiddleware.RequirePermission(authService, "workflow.view"), workflowHandler.Get)
	workflows.PUT("/:workflowID", appmiddleware.RequirePermission(authService, "workflow.manage"), workflowHandler.Define)
	workflows.DELETE("/:workflowID", appmiddleware.RequirePermission(authService, "workflow.manage"), workflowHandler.Archive)
	workflows.POST("/:workflowID/start", appmiddleware.RequirePermission(authService, "workflow.submit"), workflowHandler.Start)
	instances := v1.Group("/workflow-instances")
	instances.Use(appmiddleware.Authenticate(authService))
	instances.GET("", appmiddleware.RequirePermission(authService, "workflow.view"), workflowHandler.Instances)
	instances.POST("/:instanceID/actions", appmiddleware.RequirePermission(authService, "workflow.approve"), workflowHandler.Act)
	notifications := v1.Group("/notifications")
	notifications.Use(appmiddleware.Authenticate(authService))
	notifications.GET("", workflowHandler.Notifications)
	notifications.PUT("/:notificationID/read", workflowHandler.ReadNotification)
	knowledge := v1.Group("/knowledge")
	knowledge.Use(appmiddleware.Authenticate(authService))
	knowledge.GET("/documents", appmiddleware.RequirePermission(authService, "knowledge.view"), aiHandler.Documents)
	knowledge.POST("/documents", appmiddleware.RequirePermission(authService, "knowledge.manage"), aiHandler.Upload)
	knowledge.DELETE("/documents/:documentID", appmiddleware.RequirePermission(authService, "knowledge.manage"), aiHandler.DeleteDocument)
	knowledge.POST("/search", appmiddleware.RequirePermission(authService, "knowledge.search"), aiHandler.Search)
	ai := v1.Group("/ai")
	ai.Use(appmiddleware.Authenticate(authService), appmiddleware.RequirePermission(authService, "ai.chat"))
	ai.POST("/ask", aiHandler.Ask)
	ai.GET("/conversations", aiHandler.Conversations)
	ai.GET("/conversations/:conversationID/messages", aiHandler.Messages)
	billing := v1.Group("/billing")
	billing.Use(appmiddleware.Authenticate(authService))
	billing.GET("/plans", billingHandler.Plans)
	billing.GET("/overview", appmiddleware.RequirePermission(authService, "billing.manage"), billingHandler.Overview)
	billing.POST("/checkout", appmiddleware.RequirePermission(authService, "billing.manage"), billingHandler.Checkout)
	v1.POST("/billing/webhook", billingHandler.Webhook)
	files := v1.Group("/files")
	files.Use(appmiddleware.Authenticate(authService))
	files.GET("", appmiddleware.RequirePermission(authService, "file.view"), fileHandler.List)
	files.POST("", appmiddleware.RequirePermission(authService, "file.manage"), fileHandler.Upload)
	files.GET("/:fileID/download", appmiddleware.RequirePermission(authService, "file.view"), fileHandler.Download)
	files.DELETE("/:fileID", appmiddleware.RequirePermission(authService, "file.manage"), fileHandler.Delete)
	dashboard := v1.Group("/dashboard")
	dashboard.Use(appmiddleware.Authenticate(authService))
	dashboard.GET("", appmiddleware.RequirePermission(authService, "dashboard.view"), dashboardHandler.Get)
	dashboard.PUT("", appmiddleware.RequirePermission(authService, "dashboard.manage"), dashboardHandler.Save)

	router.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, 4004, "resource not found", nil)
	})

	return router
}
