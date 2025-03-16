package api

import (
	"net/http"

	"github.com/auditcue/integration-framework/internal/api/handlers"
	"github.com/auditcue/integration-framework/internal/api/middleware"
	"github.com/auditcue/integration-framework/internal/config"
	"github.com/auditcue/integration-framework/internal/db"
	"github.com/auditcue/integration-framework/internal/queue"
	"github.com/auditcue/integration-framework/internal/workflow"
	"github.com/auditcue/integration-framework/pkg/logger"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRouter configures and returns the API router
func SetupRouter(cfg *config.Config, database *db.Database, jobQueue *queue.Queue, logger *logger.Logger) *gin.Engine {
	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()

	// Apply middlewares
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.RateLimit(cfg))

	// Configure CORS if enabled
	if cfg.Server.EnableCORS {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowHeaders = append(corsConfig.AllowHeaders, "Authorization")
		router.Use(cors.New(corsConfig))
	}

	// Create workflow engine
	engine := workflow.NewEngine(database.DB(), logger)

	// Register service providers
	registerServiceProviders(engine, cfg)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, database, logger)
	connectionHandler := handlers.NewConnectionHandler(database, engine, logger)
	workflowHandler := handlers.NewWorkflowHandler(database, engine, jobQueue, logger)
	executionHandler := handlers.NewExecutionHandler(database, engine, logger)

	// Public routes
	public := router.Group("/api/v1")
	{
		// Health check
		public.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// Auth routes
		auth := public.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
			auth.POST("/refresh", authHandler.RefreshToken)

			// OAuth callback routes
			oauth := auth.Group("/oauth")
			{
				oauth.GET("/callback/:service", authHandler.OAuthCallback)
			}
		}
	}

	// Protected routes
	protected := router.Group("/api/v1")
	protected.Use(middleware.Auth(cfg))
	{
		// User routes
		user := protected.Group("/user")
		{
			user.GET("/profile", authHandler.GetProfile)
			user.PUT("/profile", authHandler.UpdateProfile)
			user.PUT("/password", authHandler.ChangePassword)
		}

		// Connection routes
		connections := protected.Group("/connections")
		{
			connections.GET("", connectionHandler.ListConnections)
			connections.POST("", connectionHandler.CreateConnection)
			connections.GET("/:id", connectionHandler.GetConnection)
			connections.PUT("/:id", connectionHandler.UpdateConnection)
			connections.DELETE("/:id", connectionHandler.DeleteConnection)
			connections.POST("/:id/test", connectionHandler.TestConnection)

			// OAuth routes
			oauth := connections.Group("/oauth")
			{
				oauth.GET("/:service/url", connectionHandler.GetOAuthURL)
			}
		}

		// Integration routes (available services, triggers, and actions)
		integrations := protected.Group("/integrations")
		{
			integrations.GET("", connectionHandler.ListIntegrations)
			integrations.GET("/:service", connectionHandler.GetIntegrationDetails)
			integrations.GET("/:service/triggers", connectionHandler.ListTriggers)
			integrations.GET("/:service/actions", connectionHandler.ListActions)
		}

		// Workflow routes
		workflows := protected.Group("/workflows")
		{
			workflows.GET("", workflowHandler.ListWorkflows)
			workflows.POST("", workflowHandler.CreateWorkflow)
			workflows.GET("/:id", workflowHandler.GetWorkflow)
			workflows.PUT("/:id", workflowHandler.UpdateWorkflow)
			workflows.DELETE("/:id", workflowHandler.DeleteWorkflow)
			workflows.POST("/:id/activate", workflowHandler.ActivateWorkflow)
			workflows.POST("/:id/deactivate", workflowHandler.DeactivateWorkflow)
			workflows.POST("/:id/test", workflowHandler.TestWorkflow)

			// Action routes
			actions := workflows.Group("/:id/actions")
			{
				actions.POST("", workflowHandler.AddAction)
				actions.PUT("/:actionId", workflowHandler.UpdateAction)
				actions.DELETE("/:actionId", workflowHandler.DeleteAction)
				actions.PUT("/reorder", workflowHandler.ReorderActions)
			}

			// Data mapping routes
			mappings := workflows.Group("/:id/mappings")
			{
				mappings.POST("", workflowHandler.AddDataMapping)
				mappings.PUT("/:mappingId", workflowHandler.UpdateDataMapping)
				mappings.DELETE("/:mappingId", workflowHandler.DeleteDataMapping)
			}
		}

		// Execution routes
		executions := protected.Group("/executions")
		{
			executions.GET("", executionHandler.ListExecutions)
			executions.GET("/:id", executionHandler.GetExecution)
			executions.POST("/workflow/:workflowId", executionHandler.TriggerExecution)
		}
	}

	// Webhook routes
	webhooks := router.Group("/api/v1/webhooks")
	{
		webhooks.POST("/:token", workflowHandler.HandleWebhook)
	}

	return router
}

// registerServiceProviders registers all service providers with the workflow engine
func registerServiceProviders(engine *workflow.Engine, cfg *config.Config) {
	// Register Jira service provider
	jiraProvider := jira.NewJiraProvider(cfg.Integrations.Jira)
	engine.RegisterServiceProvider(jiraProvider)

	// Register Slack service provider
	slackProvider := slack.NewSlackProvider(cfg.Integrations.Slack)
	engine.RegisterServiceProvider(slackProvider)

	// Register Teams service provider
	teamsProvider := teams.NewTeamsProvider(cfg.Integrations.Teams)
	engine.RegisterServiceProvider(teamsProvider)

	// Register Google service provider
	googleProvider := google.NewGoogleProvider(cfg.Integrations.Google)
	engine.RegisterServiceProvider(googleProvider)

	// Register Microsoft service provider
	microsoftProvider := microsoft.NewMicrosoftProvider(cfg.Integrations.Microsoft)
	engine.RegisterServiceProvider(microsoftProvider)

	// Register ServiceNow service provider
	servicenowProvider := servicenow.NewServiceNowProvider(cfg.Integrations.ServiceNow)
	engine.RegisterServiceProvider(servicenowProvider)

	// Register data transformers
	registerDataTransformers(engine)
}

// registerDataTransformers registers all data transformers with the workflow engine
func registerDataTransformers(engine *workflow.Engine) {
	// Register basic transformers
	engine.RegisterDataTransformer("toString", transformToString)
	engine.RegisterDataTransformer("toNumber", transformToNumber)
	engine.RegisterDataTransformer("toBoolean", transformToBoolean)
	engine.RegisterDataTransformer("toArray", transformToArray)
	engine.RegisterDataTransformer("toLowerCase", transformToLowerCase)
	engine.RegisterDataTransformer("toUpperCase", transformToUpperCase)
}

// Data transformer functions

func transformToString(input interface{}) (interface{}, error) {
	return fmt.Sprintf("%v", input), nil
}

func transformToNumber(input interface{}) (interface{}, error) {
	switch v := input.(type) {
	case string:
		return strconv.ParseFloat(v, 64)
	case int:
		return float64(v), nil
	case float64:
		return v, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to number", input)
	}
}

func transformToBoolean(input interface{}) (interface{}, error) {
	switch v := input.(type) {
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case bool:
		return v, nil
	default:
		return false, fmt.Errorf("cannot convert %T to boolean", input)
	}
}

func transformToArray(input interface{}) (interface{}, error) {
	switch v := input.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return []interface{}{}, nil
		}
		parts := strings.Split(v, ",")
		result := make([]interface{}, len(parts))
		for i, part := range parts {
			result[i] = strings.TrimSpace(part)
		}
		return result, nil
	case []interface{}:
		return v, nil
	default:
		return []interface{}{input}, nil
	}
}

func transformToLowerCase(input interface{}) (interface{}, error) {
	if str, ok := input.(string); ok {
		return strings.ToLower(str), nil
	}
	return fmt.Sprintf("%v", input), nil
}

func transformToUpperCase(input interface{}) (interface{}, error) {
	if str, ok := input.(string); ok {
		return strings.ToUpper(str), nil
	}
	return fmt.Sprintf("%v", input), nil
}
