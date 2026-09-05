package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	// Infrastructure
	aiOpenAI "github.com/masterfabric-go/masterfabric/internal/infrastructure/ai/openai"
	infraAuth "github.com/masterfabric-go/masterfabric/internal/infrastructure/auth"
	emailInfra "github.com/masterfabric-go/masterfabric/internal/infrastructure/email"
	graphqlInfra "github.com/masterfabric-go/masterfabric/internal/infrastructure/graphql"
	apimgmtHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/apimanagement"
	auditHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/audit"
	iamHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/iam"
	realtimeHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/realtime"
	tenantHandler "github.com/masterfabric-go/masterfabric/internal/infrastructure/http/handler/tenant"
	"github.com/masterfabric-go/masterfabric/internal/infrastructure/http/router"
	infraKafka "github.com/masterfabric-go/masterfabric/internal/infrastructure/kafka"
	pgAIAdmin "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/aiadmin"
	pgApimgmt "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/apimanagement"
	pgAudit "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/audit"
	pgEvaluation "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/evaluation"
	pgIam "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/iam"
	pgInterview "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/interview"
	pgTenant "github.com/masterfabric-go/masterfabric/internal/infrastructure/postgres/tenant"
	infraWS "github.com/masterfabric-go/masterfabric/internal/infrastructure/websocket"

	// Application use cases
	aiRuntime "github.com/masterfabric-go/masterfabric/internal/application/ai/runtime"
	aiadminUC "github.com/masterfabric-go/masterfabric/internal/application/aiadmin/usecase"
	apimgmtUC "github.com/masterfabric-go/masterfabric/internal/application/apimanagement/usecase"
	evaluationUC "github.com/masterfabric-go/masterfabric/internal/application/evaluation/usecase"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	interviewUC "github.com/masterfabric-go/masterfabric/internal/application/interview/usecase"
	realtimeUC "github.com/masterfabric-go/masterfabric/internal/application/realtime/usecase"
	tenantUC "github.com/masterfabric-go/masterfabric/internal/application/tenant/usecase"

	// AI gateway
	aiService "github.com/masterfabric-go/masterfabric/internal/domain/ai/service"
	aiadminModel "github.com/masterfabric-go/masterfabric/internal/domain/aiadmin/model"

	// Gateway
	"github.com/masterfabric-go/masterfabric/internal/gateway"
	gatewayInterceptors "github.com/masterfabric-go/masterfabric/internal/infrastructure/gateway/interceptors"

	// Shared
	"github.com/masterfabric-go/masterfabric/internal/shared/cache"
	"github.com/masterfabric-go/masterfabric/internal/shared/config"
	"github.com/masterfabric-go/masterfabric/internal/shared/database"
	"github.com/masterfabric-go/masterfabric/internal/shared/events"
	"github.com/masterfabric-go/masterfabric/internal/shared/logger"
	"github.com/masterfabric-go/masterfabric/internal/shared/telemetry"
	"github.com/masterfabric-go/masterfabric/internal/shared/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log := logger.New(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(log)

	log.Info("starting masterfabric-go",
		"host", cfg.Server.Host,
		"port", cfg.Server.Port,
		"environment", cfg.Environment,
	)

	if err := cfg.ValidateForProduction(); err != nil {
		return fmt.Errorf("invalid production configuration: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize OpenTelemetry
	otelShutdown, err := telemetry.Setup(ctx, version.ServiceName, version.Version)
	if err != nil {
		log.Warn("opentelemetry setup failed", "error", err)
	} else {
		defer func() { _ = otelShutdown(context.Background()) }()
		log.Info("opentelemetry initialized")
	}

	// Initialize PostgreSQL
	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		if cfg.IsProduction() {
			return fmt.Errorf("postgres is required in production: %w", err)
		}
		log.Warn("postgres unavailable, running without database", "error", err)
		db = nil
	} else {
		defer db.Close()
		log.Info("connected to postgres")
	}

	// Initialize Redis
	redisClient, err := cache.NewRedisClient(ctx, cfg.Redis)
	if err != nil {
		if cfg.IsProduction() {
			return fmt.Errorf("redis is required in production: %w", err)
		}
		log.Warn("redis unavailable, running without cache", "error", err)
		redisClient = nil
	} else {
		defer redisClient.Close()
		log.Info("connected to redis")
	}

	// Initialize event bus (Kafka or in-process)
	eventBus := initEventBus(ctx, cfg, log)
	defer func() { _ = eventBus.Close() }()

	// Build dependencies
	deps, err := buildDependencies(log, cfg, db, redisClient, eventBus)
	if err != nil {
		return err
	}

	// Build router
	r := router.New(deps)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", addr)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-shutdown:
		log.Info("shutdown signal received", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		log.Info("server stopped gracefully")
	}

	return nil
}

// initEventBus creates either a Kafka bus or an in-process bus based on config.
func initEventBus(ctx context.Context, cfg *config.Config, log *slog.Logger) events.EventBus {
	if !cfg.Kafka.Enabled {
		log.Info("using in-process event bus (set KAFKA_ENABLED=true to use Kafka)")
		return events.NewInProcessBus(log, 256)
	}

	log.Info("initializing kafka event bus",
		"brokers", cfg.Kafka.Brokers,
		"group_id", cfg.Kafka.GroupID,
	)

	// Ensure topics exist
	if len(cfg.Kafka.Brokers) > 0 {
		if err := infraKafka.EnsureTopics(
			ctx,
			cfg.Kafka.Brokers[0],
			infraKafka.DefaultTopics(),
			cfg.Kafka.NumPartitions,
			cfg.Kafka.ReplicationFactor,
			log,
		); err != nil {
			log.Warn("failed to ensure kafka topics, falling back to in-process bus", "error", err)
			return events.NewInProcessBus(log, 256)
		}
	}

	kafkaBus := infraKafka.NewBus(cfg.Kafka.Brokers, cfg.Kafka.GroupID, log)

	// Start consuming (after subscriptions are registered in buildDependencies)
	// We start consumption with a background context so it outlives the startup ctx.
	kafkaBus.Start(context.Background())

	log.Info("kafka event bus initialized")
	return kafkaBus
}

func buildDependencies(
	log *slog.Logger,
	cfg *config.Config,
	db *pgxpool.Pool,
	redisClient *redis.Client,
	eventBus events.EventBus,
) (router.Dependencies, error) {
	deps := router.Dependencies{
		Logger:             log,
		DB:                 db,
		Redis:              redisClient,
		CORSAllowedOrigins: cfg.Server.CORSAllowedOrigins,
		MaxBodyBytes:       cfg.Server.MaxBodyBytes,
	}

	if db == nil {
		log.Warn("database not available, API endpoints will not work")
		return deps, nil
	}

	// --- Repositories ---
	userRepo := pgIam.NewUserRepo(db)
	orgUserRepo := pgIam.NewOrgUserRepo(db)
	roleRepo := pgIam.NewRoleRepo(db)
	orgRepo := pgTenant.NewOrgRepo(db)
	workspaceRepo := pgTenant.NewWorkspaceRepository(db)
	appRepo := pgTenant.NewAppRepo(db)
	apiKeyRepo := pgTenant.NewAPIKeyRepo(db)
	endpointRepo := pgApimgmt.NewEndpointRepo(db)
	policyRepo := pgApimgmt.NewPolicyRepo(db)
	auditRepo := pgAudit.NewAuditRepo(db)
	otpRepo := pgIam.NewOTPChallengeRepo(db)
	sessionRepo := pgIam.NewSessionRepo(db)
	deviceRepo := pgIam.NewDeviceRepo(db)
	interviewRepo := pgInterview.NewRepository(db)
	evaluationRepo := pgEvaluation.NewRepository(db)
	aiadminRepo := pgAIAdmin.NewRepository(db)

	// --- Services ---
	jwtService := infraAuth.NewJWTService(cfg.JWT)
	rbacService := infraAuth.NewRBACService(roleRepo, redisClient)

	deps.AuthService = jwtService
	deps.RBACService = rbacService
	deps.OrgRepo = orgRepo
	deps.WorkspaceRepo = workspaceRepo
	deps.AppRepo = appRepo
	deps.AuditRepo = auditRepo
	var securityLimiter iamUC.RateLimiter = iamUC.NewMemoryRateLimiter()
	if redisClient != nil {
		securityLimiter = iamUC.NewRedisRateLimiter(redisClient)
	}
	emailSender, err := emailInfra.NewSender(cfg.Email)
	if err != nil {
		return deps, fmt.Errorf("configure email sender: %w", err)
	}
	securityUC := iamUC.NewSecurityService(otpRepo, sessionRepo, deviceRepo, userRepo, jwtService, emailSender, securityLimiter, iamUC.RepositoryAuditRecorder{Repo: auditRepo}, cfg.JWT.Secret, cfg.Security)
	deps.SessionValidator = securityUC

	// --- Use cases (with event bus for domain event publishing) ---
	registerUC := iamUC.NewRegisterUseCase(userRepo, jwtService, eventBus)
	loginUC := iamUC.NewLoginUseCase(userRepo, jwtService, iamUC.RepositoryAuditRecorder{Repo: auditRepo})
	organizationContextUC := iamUC.NewOrganizationContextUseCase(userRepo, orgRepo, orgUserRepo, jwtService, rbacService)
	interviewService := interviewUC.NewInterviewService(interviewRepo, jwtService, cfg.JWT.Secret)
	var interviewerGateway aiService.Gateway
	var evaluatorGateway aiService.Gateway
	if cfg.AI.Enabled {
		interviewerClient, clientErr := aiOpenAI.NewClient(&http.Client{Timeout: cfg.AI.RequestTimeout}, cfg.AI.Interviewer.BaseURL, cfg.AI.Interviewer.APIKey, cfg.AI.Interviewer.ModelID, cfg.AI.Interviewer.ModelVersion)
		if clientErr != nil {
			return deps, fmt.Errorf("configure interviewer AI gateway: %w", clientErr)
		}
		interviewerClient.SetMaxResponseBytes(cfg.AI.MaxResponseBytes)
		interviewerGateway = interviewerClient
		log.Info("interviewer AI gateway configured", "model", cfg.AI.Interviewer.ModelID, "endpoint", cfg.AI.Interviewer.BaseURL)
		evaluatorClient, clientErr := aiOpenAI.NewClient(&http.Client{Timeout: cfg.AI.RequestTimeout}, cfg.AI.Evaluator.BaseURL, cfg.AI.Evaluator.APIKey, cfg.AI.Evaluator.ModelID, cfg.AI.Evaluator.ModelVersion)
		if clientErr != nil {
			return deps, fmt.Errorf("configure evaluator AI gateway: %w", clientErr)
		}
		evaluatorClient.SetMaxResponseBytes(cfg.AI.MaxResponseBytes)
		evaluatorGateway = evaluatorClient
		log.Info("evaluator AI gateway configured", "model", cfg.AI.Evaluator.ModelID, "endpoint", cfg.AI.Evaluator.BaseURL)
	}
	var runtimeGateway aiService.Gateway
	if cfg.AI.Enabled {
		runtimeGateway = aiRuntime.NewRegistryGateway(aiadminRepo, map[string]aiService.Gateway{
			cfg.AI.Interviewer.ProviderLabel: interviewerGateway,
			cfg.AI.Evaluator.ProviderLabel:   evaluatorGateway,
		}, map[aiadminModel.Role]aiService.Gateway{
			aiadminModel.RoleInterviewer: interviewerGateway,
			aiadminModel.RoleEvaluator:   evaluatorGateway,
		})
	}
	questionDraftService := interviewUC.NewQuestionDraftService(interviewRepo, interviewRepo, runtimeGateway)
	evaluationService := evaluationUC.NewEvaluationService(evaluationRepo, interviewRepo, runtimeGateway)
	aiadminService := aiadminUC.NewService(aiadminRepo, auditRepo, cfg.Security)
	assignRoleUC := iamUC.NewAssignRoleUseCase(roleRepo, orgUserRepo, appRepo, rbacService, eventBus)
	createOrgUC := tenantUC.NewCreateOrgUseCase(orgRepo, eventBus)
	createWorkspaceUC := tenantUC.NewCreateWorkspaceUseCase(workspaceRepo, orgRepo, eventBus)
	listWorkspacesUC := tenantUC.NewListWorkspacesUseCase(workspaceRepo)
	updateWorkspaceUC := tenantUC.NewUpdateWorkspaceUseCase(workspaceRepo)
	createAppUC := tenantUC.NewCreateAppUseCase(appRepo, orgRepo, eventBus)
	manageKeysUC := tenantUC.NewManageAPIKeysUseCase(apiKeyRepo)
	defineEndpointUC := apimgmtUC.NewDefineEndpointUseCase(endpointRepo, eventBus)
	updatePolicyUC := apimgmtUC.NewUpdatePolicyUseCase(policyRepo)
	retireEndpointUC := apimgmtUC.NewRetireEndpointUseCase(endpointRepo, eventBus)
	activateEndpointUC := apimgmtUC.NewActivateEndpointUseCase(endpointRepo, eventBus)

	// --- Register sample Kafka consumers ---
	// Log all IAM events
	eventBus.Subscribe(events.TopicIAM, func(ctx context.Context, event events.Event) error {
		log.Info("iam event received", "event", event)
		return nil
	})
	// Log all tenant events
	eventBus.Subscribe(events.TopicTenant, func(ctx context.Context, event events.Event) error {
		log.Info("tenant event received", "event", event)
		return nil
	})
	// Log all API management events
	eventBus.Subscribe(events.TopicAPIManagement, func(ctx context.Context, event events.Event) error {
		log.Info("api-management event received", "event", event)
		return nil
	})

	// --- Handlers ---
	deps.IAMHandler = iamHandler.NewHandler(registerUC, loginUC, assignRoleUC, userRepo, securityUC)
	deps.TenantHandler = tenantHandler.NewHandler(
		createOrgUC,
		createAppUC,
		manageKeysUC,
		createWorkspaceUC,
		listWorkspacesUC,
		updateWorkspaceUC,
		orgRepo,
		appRepo,
	)
	deps.APIMgmtHandler = apimgmtHandler.NewHandler(defineEndpointUC, updatePolicyUC, retireEndpointUC, activateEndpointUC, endpointRepo, policyRepo)
	deps.AuditHandler = auditHandler.NewHandler(auditRepo)
	deps.GraphQLHandler = graphqlInfra.NewHandler(graphqlInfra.Dependencies{
		AuthService:          jwtService,
		SessionValidator:     securityUC,
		UseCase:              organizationContextUC,
		InterviewUseCase:     interviewService,
		EvaluationUseCase:    evaluationService,
		QuestionDraftUseCase: questionDraftService,
		AIAdminUseCase:       aiadminService,
		Timeout:              5 * time.Second,
		MaxBodyBytes:         cfg.Server.MaxBodyBytes,
	})

	// --- WebSocket real-time hub ---
	wsHub := infraWS.NewHub(log, cfg.WebSocket.MaxConnections)
	eventBridge := infraWS.NewEventBridge(wsHub, appRepo, log)
	eventBridge.Register(eventBus)

	validateConnectUC := realtimeUC.NewValidateConnectUseCase(appRepo, rbacService)
	wsUpgrader := infraWS.NewUpgrader(infraWS.UpgraderConfig{
		ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
		WriteBufferSize: cfg.WebSocket.WriteBufferSize,
		AllowedOrigins:  cfg.Server.CORSAllowedOrigins,
	})
	deps.RealtimeHandler = realtimeHandler.NewHandler(realtimeHandler.Config{
		ValidateUC:   validateConnectUC,
		AuthService:  jwtService,
		Hub:          wsHub,
		Upgrader:     wsUpgrader,
		PingInterval: cfg.WebSocket.PingIntervalSec,
		Logger:       log,
		Enabled:      cfg.WebSocket.Enabled,
	})

	// --- Gateway pipeline with interceptors ---
	// Create interceptor chain: schema validation, PII masking, request/response transformers
	piiMasker := gatewayInterceptors.NewPIIMasker(
		[]string{"password", "password_hash", "api_key", "secret", "token", "ssn", "credit_card"},
		"***",
	)
	schemaValidator := gatewayInterceptors.NewSchemaValidator()

	// Create dynamic handler resolver for routing requests to backend service handlers
	// This supports:
	// 1. Registered handlers (if you register specific handlers)
	// 2. HTTP proxy to external services (if backend_service is a URL or configured)
	// 3. Generic dynamic database handler (automatically performs CRUD operations)
	backendRegistry := gateway.NewBackendRegistry()
	dynamicResolver := gateway.NewDynamicHandlerResolver(backendRegistry, log, db)

	// Optional: Register service configurations for HTTP proxying
	// Example:
	// dynamicResolver.RegisterServiceConfig("product-service", gateway.ServiceConfig{
	//     BaseURL: "https://api.example.com/products",
	//     Headers: map[string]string{"Authorization": "Bearer token"},
	// })

	// Optional: Register specific handlers for services that need custom logic
	// Example:
	// productHandler := handlers.NewProductHandler(...)
	// backendRegistry.Register("product-service", productHandler)

	// Wire interceptors into gateway pipeline with dynamic resolver
	deps.GatewayPipeline = gateway.NewPipeline(
		endpointRepo,
		policyRepo,
		rbacService,
		redisClient,
		log,
		dynamicResolver, // Dynamic handler resolver (supports registered handlers, HTTP proxy, and generic handling)
		schemaValidator, // Schema validation interceptor
		piiMasker,       // PII masking interceptor
	)

	return deps, nil
}
