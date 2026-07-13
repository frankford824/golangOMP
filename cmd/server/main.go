package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql" // registers mysql driver
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/config"
	"workflow/policy"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	aiagentsvc "workflow/service/aiagent"
	assetcenter "workflow/service/asset_center"
	assetlifecycle "workflow/service/asset_lifecycle"
	"workflow/service/asset_lifecycle/scheduler"
	assetworkbench "workflow/service/asset_workbench"
	"workflow/service/blueprint"
	designsourcesvc "workflow/service/design_source"
	erpproductsvc "workflow/service/erp_product"
	externalassets "workflow/service/external_assets"
	r3module "workflow/service/module_action"
	notificationsvc "workflow/service/notification"
	orgmovesvc "workflow/service/org_move_request"
	predictionsvc "workflow/service/prediction"
	reportl1svc "workflow/service/report_l1"
	searchsvc "workflow/service/search"
	"workflow/service/task_aggregator"
	taskaisummarysvc "workflow/service/task_ai_summary"
	taskbatchexcel "workflow/service/task_batch_excel"
	"workflow/service/task_cancel"
	taskdraftsvc "workflow/service/task_draft"
	tasklifecycle "workflow/service/task_lifecycle"
	"workflow/service/task_pool"
	tasksingleexcel "workflow/service/task_single_excel"
	wsservice "workflow/service/websocket"
	wecombotsvc "workflow/service/wecombot"
	"workflow/transport"
	"workflow/transport/handler"
	transportws "workflow/transport/ws"
	"workflow/workers"
)

func main() {
	// ── 1. Config (12-Factor: env vars with sane defaults) ───────────────────
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Structured logger (JSON in prod, console in debug) ─────────────────
	logger := buildLogger(cfg.Log.Level)
	defer logger.Sync() //nolint:errcheck
	logger.Info("task org catalog bootstrap", zap.Int("department_team_count", len(cfg.Auth.DepartmentTeams)), zap.Strings("department_keys", sortedTaskOrgDepartmentKeys(cfg.Auth.DepartmentTeams)))
	service.ConfigureTaskOrgCatalog(cfg.Auth)

	// ── 3. MySQL ──────────────────────────────────────────────────────────────
	//      DSN env: MYSQL_DSN=user:pass@tcp(host:3306)/workflow?charset=utf8mb4&parseTime=True&loc=Local
	db, err := connectMySQL(cfg.MySQL)
	if err != nil {
		logger.Fatal("MySQL connect failed", zap.Error(err))
	}
	defer db.Close()
	logger.Info("MySQL connected")

	// ── 4. Redis ──────────────────────────────────────────────────────────────
	//      Used for: idempotency tokens, leases, rate-limiting, WS fan-out pub/sub
	//      Env: REDIS_ADDR, REDIS_PASSWORD, REDIS_DB
	rdb, err := connectRedis(cfg.Redis)
	if err != nil {
		logger.Fatal("Redis connect failed", zap.Error(err))
	}
	defer rdb.Close()
	logger.Info("Redis connected", zap.String("addr", cfg.Redis.Addr))

	// ── 5. Wire: repos → services → handlers ──────────────────────────────────
	mdb := mysqlrepo.New(db)
	skuRepo := mysqlrepo.NewSKURepo(mdb)
	eventRepo := mysqlrepo.NewEventRepo(mdb)
	assetVersionRepo := mysqlrepo.NewAssetVersionRepo(mdb)
	auditRepo := mysqlrepo.NewAuditRepo(mdb)
	jobRepo := mysqlrepo.NewJobRepo(mdb)
	incidentRepo := mysqlrepo.NewIncidentRepo(mdb)
	policyRepo := mysqlrepo.NewPolicyRepo(mdb)
	engine := policy.NewEngine()

	// V7 repos
	userRepo := mysqlrepo.NewUserRepo(mdb)
	orgRepo := mysqlrepo.NewOrgRepo(mdb)
	userSessionRepo := mysqlrepo.NewUserSessionRepo(mdb)
	permissionLogRepo := mysqlrepo.NewPermissionLogRepo(mdb)
	productRepo := mysqlrepo.NewProductRepo(mdb)
	productManagementRepo := mysqlrepo.NewProductManagementRepo(mdb)
	categoryRepo := mysqlrepo.NewCategoryRepo(mdb)
	categoryERPMappingRepo := mysqlrepo.NewCategoryERPMappingRepo(mdb)
	costRuleRepo := mysqlrepo.NewCostRuleRepo(mdb)
	costRuleBindingRepo := mysqlrepo.NewCostRuleBindingRepo(mdb)
	costRecalculationRunRepo := mysqlrepo.NewCostRecalculationRunRepo(mdb)
	erpSyncRunRepo := mysqlrepo.NewERPSyncRunRepo(mdb)
	taskRepo := mysqlrepo.NewTaskRepo(mdb)
	taskCreateRequestRepo := mysqlrepo.NewTaskCreateRequestRepo(mdb)
	skuTraceRepo := mysqlrepo.NewSKUTraceRepo(mdb)
	skuComboRepo := mysqlrepo.NewSKUComboRepo(mdb)
	procurementRepo := mysqlrepo.NewProcurementRepo(mdb)
	taskCostOverrideEventRepo := mysqlrepo.NewTaskCostOverrideEventRepo(mdb)
	taskCostOverrideReviewRepo := mysqlrepo.NewTaskCostOverrideReviewRepo(mdb)
	taskCostFinanceFlagRepo := mysqlrepo.NewTaskCostFinanceFlagRepo(mdb)
	workbenchPreferenceRepo := mysqlrepo.NewWorkbenchPreferenceRepo(mdb)
	exportJobRepo := mysqlrepo.NewExportJobRepo(mdb)
	exportJobDispatchRepo := mysqlrepo.NewExportJobDispatchRepo(mdb)
	exportJobAttemptRepo := mysqlrepo.NewExportJobAttemptRepo(mdb)
	exportJobEventRepo := mysqlrepo.NewExportJobEventRepo(mdb)
	designAssetRepo := mysqlrepo.NewDesignAssetRepo(mdb)
	uploadRequestRepo := mysqlrepo.NewUploadRequestRepo(mdb)
	assetStorageRefRepo := mysqlrepo.NewAssetStorageRefRepo(mdb)
	integrationCallLogRepo := mysqlrepo.NewIntegrationCallLogRepo(mdb)
	integrationExecutionRepo := mysqlrepo.NewIntegrationExecutionRepo(mdb)
	codeRuleRepo := mysqlrepo.NewCodeRuleRepo(mdb)
	productCodeSeqRepo := mysqlrepo.NewProductCodeSequenceRepo(mdb)
	ruleTemplateRepo := mysqlrepo.NewRuleTemplateRepo(mdb)
	serverLogRepo := mysqlrepo.NewServerLogRepo(mdb)
	auditV7Repo := mysqlrepo.NewAuditV7Repo(mdb)
	outsourceRepo := mysqlrepo.NewOutsourceRepo(mdb)
	taskAssetRepo := mysqlrepo.NewTaskAssetRepo(mdb)
	taskEventRepo := mysqlrepo.NewTaskEventRepo(mdb)
	warehouseRepo := mysqlrepo.NewWarehouseRepo(mdb)
	customizationJobRepo := mysqlrepo.NewCustomizationJobRepo(mdb)
	customizationPricingRuleRepo := mysqlrepo.NewCustomizationPricingRuleRepo(mdb)
	taskModuleRepo := mysqlrepo.NewTaskModuleRepo(mdb)
	taskModuleEventRepo := mysqlrepo.NewTaskModuleEventRepo(mdb)
	referenceFileRefFlatRepo := mysqlrepo.NewReferenceFileRefFlatRepo(mdb)
	taskRetouchRequirementRepo := mysqlrepo.NewTaskRetouchRequirementRepo(mdb)
	taskReferenceAssetBindingRepo := mysqlrepo.NewTaskReferenceAssetBindingRepo(mdb)
	taskAssetSearchRepo := mysqlrepo.NewTaskAssetSearchRepo(mdb)
	taskAssetLifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mdb)
	externalAssetRepo := mysqlrepo.NewExternalAssetRepo(mdb)
	taskAutoArchiveRepo := mysqlrepo.NewTaskAutoArchiveRepo(mdb)
	orgMoveRequestRepo := mysqlrepo.NewOrgMoveRequestRepo(mdb)
	taskDraftRepo := mysqlrepo.NewTaskDraftRepo(mdb)
	notificationRepo := mysqlrepo.NewNotificationRepo(mdb)
	designSourceRepo := mysqlrepo.NewDesignSourceRepo(mdb)
	moduleNotificationRepo := mysqlrepo.NewModuleNotificationRepo(mdb)
	searchRepo := mysqlrepo.NewSearchRepo(mdb)
	predictionRepo := mysqlrepo.NewPredictionRepo(mdb)
	reportL1Repo := mysqlrepo.NewReportL1Repo(mdb)
	taskOperationalDashboardRepo := mysqlrepo.NewTaskOperationalDashboardRepo(mdb)
	kpiAnalysisRepo := mysqlrepo.NewKPIAnalysisRepo(mdb)
	businessTrendRepo := mysqlrepo.NewBusinessTrendRepo(mdb)
	workflowTraceEventRepo := mysqlrepo.NewWorkflowTraceEventRepo(mdb)
	assetWorkbenchRepo := mysqlrepo.NewAssetWorkbenchRepo(mdb)
	experienceRepo := mysqlrepo.NewExperienceRepo(mdb)

	skuSvc := service.NewSKUService(skuRepo, eventRepo, mdb, engine)
	auditSvc := service.NewAuditService(auditRepo, skuRepo, assetVersionRepo, jobRepo, eventRepo, incidentRepo, policyRepo, mdb, engine)
	agentSvc := service.NewAgentService(assetVersionRepo, skuRepo, jobRepo, eventRepo, incidentRepo, policyRepo, mdb, engine)
	incidentSvc := service.NewIncidentService(incidentRepo, eventRepo, mdb)
	policySvc := service.NewPolicyService(policyRepo)
	identitySvc := service.NewIdentityService(userRepo, userSessionRepo, permissionLogRepo, mdb, service.WithIdentitySettings(cfg.Auth, cfg.FrontendAccess), service.WithOrgRepo(orgRepo), service.WithIdentityLogger(logger))
	orgMoveSvc := orgmovesvc.NewService(userRepo, orgRepo, orgMoveRequestRepo, permissionLogRepo, mdb)
	if appErr := identitySvc.SyncConfiguredAuth(context.Background()); appErr != nil {
		logger.Fatal("sync configured auth failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
	}

	// V7 services
	codeRuleSvc := service.NewCodeRuleService(codeRuleRepo, mdb)
	blueprintRegistry := blueprint.NewRegistry()
	blueprintRules := blueprint.NewRuleEngine(blueprintRegistry, taskModuleRepo, taskModuleEventRepo, taskRepo)
	categorySvc := service.NewCategoryService(categoryRepo, mdb)
	categoryMappingSvc := service.NewCategoryERPMappingService(categoryERPMappingRepo, categoryRepo, mdb)
	costRuleSvc := service.NewCostRuleService(costRuleRepo, categoryRepo, mdb)
	costRuleBindingSvc := service.NewCostRuleBindingService(costRuleBindingRepo, costRuleRepo, mdb)
	productSvc := service.NewProductService(productRepo, categoryRepo, categoryERPMappingRepo)
	var erpBridgeClient service.ERPBridgeClient
	localERPBridgeClient := service.NewLocalERPBridgeClient(productRepo, categoryRepo, mdb, integrationCallLogRepo)
	erpMode := strings.ToLower(strings.TrimSpace(cfg.ERPRemote.Mode))
	if erpMode == "" {
		erpMode = "local"
	}

	// Main(8080) keeps forwarding to Bridge(8081) HTTP as before.
	if cfg.Server.Port != "8081" {
		erpBridgeClient, err = service.NewERPBridgeClient(service.ERPBridgeClientConfig{
			BaseURL: cfg.ERPBridge.BaseURL,
			Timeout: cfg.ERPBridge.Timeout,
			Logger:  logger.Named("erp_bridge"),
		})
		if err != nil {
			logger.Fatal("ERP Bridge client config failed", zap.Error(err))
		}
	} else {
		switch erpMode {
		case "local":
			erpBridgeClient = localERPBridgeClient
		case "remote":
			erpBridgeClient, err = service.NewRemoteERPBridgeClient(erpRemoteServiceConfig(cfg, logger.Named("erp_remote")))
			if err != nil {
				logger.Fatal("ERP remote client config failed", zap.Error(err))
			}
		case "hybrid":
			remoteClient, remoteErr := service.NewRemoteERPBridgeClient(erpRemoteServiceConfig(cfg, logger.Named("erp_remote")))
			if remoteErr != nil {
				logger.Fatal("ERP remote client config failed", zap.Error(remoteErr))
			}
			erpBridgeClient = service.NewHybridERPBridgeClient(localERPBridgeClient, remoteClient, cfg.ERPRemote.FallbackToLocalOnError, logger.Named("erp_bridge_hybrid"))
		default:
			logger.Fatal("unsupported ERP_REMOTE_MODE", zap.String("mode", erpMode))
		}
		if (erpMode == "remote" || erpMode == "hybrid") && strings.TrimSpace(cfg.ERPRemote.BaseURL) == "" {
			logger.Fatal("8081 Bridge: ERP_REMOTE_BASE_URL is required when ERP_REMOTE_MODE is remote or hybrid")
		}
		if (erpMode == "remote" || erpMode == "hybrid") && !strings.EqualFold(strings.TrimSpace(cfg.ERPRemote.AuthMode), "openweb") {
			logger.Fatal("8081 Bridge: ERP_REMOTE_AUTH_MODE must be openweb when ERP_REMOTE_MODE is remote or hybrid (live OpenWeb SKU query)",
				zap.String("auth_mode", cfg.ERPRemote.AuthMode))
		}
		if erpMode == "hybrid" {
			logger.Info("8081 Bridge hybrid ERP: remote OpenWeb first; local products only on transient upstream failure (see erp_bridge_product_search logs)",
				zap.Bool("fallback_enabled", cfg.ERPRemote.FallbackToLocalOnError))
		}
		if erpMode == "remote" {
			logger.Info("8081 Bridge remote ERP: product search/detail use OpenWeb only (ERP_REMOTE_SKU_QUERY_PATH)",
				zap.String("sku_query_path", strings.TrimSpace(cfg.ERPRemote.SkuQueryPath)))
		}
	}
	erpBridgeSvc := service.NewERPBridgeService(erpBridgeClient, productRepo, mdb)
	productManagementERPBridgeSvc := erpBridgeSvc
	if cfg.Server.Port != "8081" &&
		strings.TrimSpace(cfg.ERPRemote.BaseURL) != "" &&
		strings.EqualFold(strings.TrimSpace(cfg.ERPRemote.AuthMode), "openweb") {
		productManagementRemoteClient, remoteErr := service.NewRemoteERPBridgeClient(erpRemoteServiceConfig(cfg, logger.Named("product_management_erp_remote")))
		if remoteErr != nil {
			logger.Warn("product management direct ERP client disabled", zap.Error(remoteErr))
		} else {
			productManagementClient := service.NewHybridERPBridgeClient(
				localERPBridgeClient,
				productManagementRemoteClient,
				cfg.ERPRemote.FallbackToLocalOnError,
				logger.Named("product_management_erp"),
			)
			productManagementERPBridgeSvc = service.NewERPBridgeService(productManagementClient, productRepo, mdb)
			logger.Info("Product management ERP sync uses direct OpenWeb client for background jobs")
		}
	}
	var erpProvider service.ERPProductProvider
	switch strings.ToLower(strings.TrimSpace(cfg.ERP.SourceMode)) {
	case "jst", "jst_openweb", "remote_jst":
		erpProvider = service.NewJSTOpenWebProductProvider(erpRemoteServiceConfig(cfg, logger.Named("erp_sync_jst")))
	default:
		erpProvider = service.NewStubERPProductProvider(cfg.ERP.StubFile)
	}
	erpSyncSvc := service.NewERPSyncService(productRepo, erpSyncRunRepo, mdb, erpProvider, service.ERPSyncOptions{
		SchedulerEnabled: cfg.ERP.Enabled,
		Interval:         cfg.ERP.Interval,
		SourceMode:       cfg.ERP.SourceMode,
		StubFile:         cfg.ERP.StubFile,
		Timeout:          cfg.ERP.Timeout,
		Logger:           logger.Named("erp_sync"),
	})
	taskDataScopeResolver := service.NewRoleBasedDataScopeResolver()
	taskReferenceAssetFormalizer := service.NewTaskReferenceAssetFormalizer(
		designAssetRepo,
		taskAssetRepo,
		assetStorageRefRepo,
		taskReferenceAssetBindingRepo,
		taskEventRepo,
		mdb,
	)
	ossDirectSvc := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         cfg.OSSDirect.Enabled,
		Endpoint:        cfg.OSSDirect.Endpoint,
		Bucket:          cfg.OSSDirect.Bucket,
		AccessKeyID:     cfg.OSSDirect.AccessKeyID,
		AccessKeySecret: cfg.OSSDirect.AccessKeySecret,
		PresignExpiry:   cfg.OSSDirect.PresignExpiry,
		HTTPTimeout:     cfg.OSSDirect.HTTPTimeout,
		PublicEndpoint:  cfg.OSSDirect.PublicEndpoint,
		PartSize:        cfg.OSSDirect.PartSize,
	})
	if ossDirectSvc.Enabled() {
		logger.Info("OSS direct presign service enabled",
			zap.String("bucket", cfg.OSSDirect.Bucket),
			zap.String("endpoint", cfg.OSSDirect.Endpoint))
	}
	erpImageProxySigner := service.NewERPImageProxySigner(service.ERPImageProxyConfig{
		PublicBaseURL: cfg.ERPImageProxy.PublicBaseURL,
		SigningSecret: cfg.ERPImageProxy.SigningSecret,
		TokenTTL:      cfg.ERPImageProxy.TokenTTL,
	})
	if erpImageProxySigner.Enabled() {
		logger.Info("ERP product image short proxy enabled",
			zap.String("public_base_url", cfg.ERPImageProxy.PublicBaseURL),
			zap.Duration("token_ttl", cfg.ERPImageProxy.TokenTTL))
	}
	externalAssetSvc := externalassets.NewService(externalAssetRepo, externalassets.ConfigFromApp(cfg.ExternalAssets), ossDirectSvc)
	uploadClient := service.NewUploadServiceClient(service.UploadServiceClientConfig{
		Enabled:                 cfg.UploadService.Enabled,
		BaseURL:                 cfg.UploadService.BaseURL,
		BrowserMultipartBaseURL: cfg.UploadService.BrowserMultipartBaseURL,
		BrowserDownloadBaseURL:  cfg.UploadService.BrowserDownloadBaseURL,
		Timeout:                 cfg.UploadService.Timeout,
		InternalToken:           cfg.UploadService.InternalToken,
		StorageProvider:         cfg.UploadService.StorageProvider,
	})
	wsHub := wsservice.NewHub(logger.Named("websocket"))
	wecomSender := wecombotsvc.NewSender(wecombotsvc.Config{
		Enabled:       cfg.WeCom.AiBotEnabled,
		BotID:         cfg.WeCom.AiBotBotID,
		Secret:        cfg.WeCom.AiBotSecret,
		DefaultChatID: cfg.WeCom.AiBotDefaultChatID,
		WSURL:         cfg.WeCom.AiBotWSURL,
		QueueSize:     cfg.WeCom.AiBotQueueSize,
	}, logger.Named("wecom_aibot"))
	wecomNotifier := notificationsvc.NewWeComNotifier(wecomSender, taskRepo, userRepo, logger.Named("wecom_notification"))
	notificationSvc := notificationsvc.NewService(notificationRepo, permissionLogRepo, wsHub, logger.Named("notification"),
		notificationsvc.WithUserRepo(userRepo),
		notificationsvc.WithTaskRepo(taskRepo),
		notificationsvc.WithTxRunner(mdb),
		notificationsvc.WithExternalNotifier(wecomNotifier),
		notificationsvc.WithWebPushConfig(notificationsvc.WebPushConfig{
			Enabled:     cfg.WebPush.Enabled,
			PublicKey:   cfg.WebPush.VAPIDPublicKey,
			PrivateKey:  cfg.WebPush.VAPIDPrivateKey,
			Subject:     cfg.WebPush.VAPIDSubject,
			LeaseTTL:    cfg.WebPush.LeaseTTL,
			RetryBase:   cfg.WebPush.RetryBaseDelay,
			MaxAttempts: cfg.WebPush.MaxAttempts,
		}))
	productManagementSvc := service.NewProductManagementService(productManagementRepo, taskAssetRepo, taskAssetSearchRepo, mdb,
		service.WithProductManagementERPBridge(productManagementERPBridgeSvc),
		service.WithProductManagementAssetURLServices(ossDirectSvc, uploadClient),
		service.WithProductManagementERPImageProxy(erpImageProxySigner),
		service.WithProductManagementTaskEventRepo(taskEventRepo),
		service.WithProductManagementSKUComboRepo(skuComboRepo),
		service.WithProductManagementCostRecalculationRunRepo(costRecalculationRunRepo),
		service.WithProductManagementCostLegacyAliasFallbackEnabled(cfg.CostGovernance.LegacyAliasFallbackEnabled),
		service.WithProductManagementRedis(rdb),
		service.WithProductManagementNotificationService(notificationSvc))
	costRecalculationSvc := service.NewCostRecalculationService(productManagementRepo, costRecalculationRunRepo, taskRepo, costRuleRepo, skuTraceRepo, mdb,
		service.WithCostRecalculationLegacyAliasFallbackEnabled(cfg.CostGovernance.LegacyAliasFallbackEnabled),
		service.WithCostRecalculationProductManagementRedis(rdb))
	skuComboSyncSvc := service.NewSKUComboSyncService(productManagementERPBridgeSvc, skuComboRepo, mdb)
	taskSvc := service.NewTaskServiceWithCatalog(taskRepo, procurementRepo, taskAssetRepo, taskEventRepo, taskCostOverrideEventRepo, warehouseRepo, categoryRepo, costRuleRepo, codeRuleSvc, mdb,
		service.WithTaskCostOverridePlaceholderRepos(taskCostOverrideReviewRepo, taskCostFinanceFlagRepo),
		service.WithERPBridgeSelectionBinding(erpBridgeSvc),
		service.WithTaskERPBridgeFilingTrace(integrationCallLogRepo),
		service.WithTaskSKUTraceRepo(skuTraceRepo),
		service.WithTaskReferenceFileRefValidation(uploadRequestRepo, assetStorageRefRepo),
		service.WithTaskReferenceFileRefFlatRepo(referenceFileRefFlatRepo),
		service.WithTaskReferenceAssetFormalizer(taskReferenceAssetFormalizer),
		service.WithTaskReferenceFileRefsOSSDirectService(ossDirectSvc),
		service.WithTaskDesignAssetReadModel(designAssetRepo),
		service.WithTaskProductCodeSequenceRepo(productCodeSeqRepo),
		service.WithTaskCostRuleBindingRepo(costRuleBindingRepo),
		service.WithTaskCostLegacyAliasFallbackEnabled(cfg.CostGovernance.LegacyAliasFallbackEnabled),
		service.WithTaskCreateRequestRepo(taskCreateRequestRepo),
		service.WithTaskCreateFilingAsync(),
		service.WithTaskCustomizationJobRepo(customizationJobRepo),
		service.WithTaskCustomizationReviewModuleSync(taskModuleRepo, taskModuleEventRepo),
		service.WithTaskCustomizationPricingRuleRepo(customizationPricingRuleRepo),
		service.WithUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskDataScopeResolver(taskDataScopeResolver),
		service.WithTaskScopeUserRepo(userRepo),
		service.WithTaskBlueprintRuleEngine(blueprintRules),
		service.WithTaskRetouchRequirementRepo(taskRetouchRequirementRepo),
		service.WithTaskProductManagementCloseSyncer(productManagementSvc),
		service.WithTaskNotificationService(notificationSvc))
	taskBoardSvc := service.NewTaskBoardService(taskSvc, taskOperationalDashboardRepo)
	taskBatchTemplateSvc := taskbatchexcel.NewTemplateService()
	workbenchSvc := service.NewWorkbenchService(workbenchPreferenceRepo)
	exportCenterSvc := service.NewExportCenterService(exportJobRepo, exportJobDispatchRepo, exportJobAttemptRepo, exportJobEventRepo, mdb)
	integrationCenterSvc := service.NewIntegrationCenterService(integrationCallLogRepo, integrationExecutionRepo, mdb)
	taskAssetSvc := service.NewTaskAssetService(taskRepo, taskAssetRepo, taskEventRepo, uploadRequestRepo, assetStorageRefRepo, mdb,
		service.WithTaskAssetModuleRepo(taskModuleRepo),
		service.WithTaskAssetCustomizationJobRepo(customizationJobRepo),
		service.WithTaskAssetBlueprintRuleEngine(blueprintRules),
		service.WithTaskAssetDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssetScopeUserRepo(userRepo),
		service.WithTaskAssetUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	assetUploadSvc := service.NewAssetUploadService(taskRepo, uploadRequestRepo, mdb)
	taskCreateReferenceUploadSvc := service.NewTaskCreateReferenceUploadService(
		uploadRequestRepo,
		assetStorageRefRepo,
		mdb,
		uploadClient,
		service.WithTaskCreateReferenceOSSDirectService(ossDirectSvc),
	)
	taskBatchParseSvc := taskbatchexcel.NewParseServiceWithDependencies(taskCreateReferenceUploadSvc, erpBridgeSvc)
	taskSingleTemplateSvc := tasksingleexcel.NewTemplateService()
	taskSingleParseSvc := tasksingleexcel.NewParseServiceWithDependencies(erpBridgeSvc)
	taskAssetCenterSvc := service.NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, assetStorageRefRepo, taskEventRepo, mdb, uploadClient,
		service.WithOSSDirectService(ossDirectSvc),
		service.WithTaskAssetCenterModuleRepo(taskModuleRepo),
		service.WithTaskAssetCenterCustomizationJobRepo(customizationJobRepo),
		service.WithTaskAssetCenterBlueprintRuleEngine(blueprintRules),
		service.WithTaskAssetCenterDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssetCenterScopeUserRepo(userRepo),
		service.WithTaskAssetCenterUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskAssetCenterRetouchRequirementRepo(taskRetouchRequirementRepo),
		service.WithTaskAssetCenterReferenceFileRefFlatRepo(referenceFileRefFlatRepo),
		service.WithTaskAssetCenterAuditRepo(auditV7Repo))
	globalAssetCenterSvc := assetcenter.NewService(taskAssetSearchRepo, ossDirectSvc, uploadClient, assetcenter.WithAssetCenterRedis(rdb))
	globalAssetCenterSvc.SetStorageStreamOpener(service.NewStorageStreamOpener(ossDirectSvc, uploadClient))
	globalAssetCenterSvc.SetExternalAssetService(externalAssetSvc)
	globalAssetLifecycleSvc := assetlifecycle.NewService(taskAssetSearchRepo, taskAssetLifecycleRepo, mdb, ossDirectSvc)
	taskDetailSvc := service.NewTaskDetailAggregateService(taskRepo, procurementRepo, productRepo, costRuleRepo, auditV7Repo, outsourceRepo, taskAssetRepo, warehouseRepo, taskEventRepo, taskCostOverrideEventRepo, taskCostOverrideReviewRepo, taskCostFinanceFlagRepo,
		service.WithTaskDetailScopeUserRepo(userRepo),
		service.WithTaskDetailUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskDetailDesignAssetReadModel(designAssetRepo))
	taskCostOverrideSvc := service.NewTaskCostOverrideAuditService(taskRepo, taskCostOverrideEventRepo, taskEventRepo, taskCostOverrideReviewRepo, taskCostFinanceFlagRepo)
	experienceSvc := service.NewExperienceService(experienceRepo, service.ExperienceServiceConfig{
		UIEnabled:                    cfg.Experience.UIEnabled,
		CaptureEnabled:               cfg.Experience.CaptureEnabled,
		AIFeedbackEnabled:            cfg.Experience.AIFeedbackEnabled,
		BehaviorCaptureEnabled:       cfg.Experience.BehaviorCaptureEnabled,
		MicroQuestionEnabled:         cfg.Experience.MicroQuestionEnabled,
		ReviewMaterializationEnabled: cfg.Experience.ReviewMaterializationEnabled,
		BehaviorSampleRate:           cfg.Experience.BehaviorSampleRate,
		EnabledSurfaces:              cfg.Experience.EnabledSurfaces,
		WorkerEnabled:                cfg.Experience.WorkerEnabled,
		WorkerBatchSize:              cfg.Experience.WorkerBatchSize,
		WorkerMaxAttempts:            cfg.Experience.WorkerMaxAttempts,
		OutboxLeaseTTL:               cfg.Experience.OutboxLeaseTTL,
		RuntimeConfigFile:            cfg.Experience.RuntimeConfigFile,
		RetentionDays:                cfg.Experience.RetentionDays,
	}, logger.Named("experience"))
	auditV7Options := []service.AuditV7ServiceOption{
		service.WithAuditV7DataScopeResolver(taskDataScopeResolver),
		service.WithAuditV7ScopeUserRepo(userRepo),
		service.WithAuditV7FilingTrigger(taskSvc),
		service.WithAuditV7ExperienceService(experienceSvc),
	}
	if assetFlowRepo, ok := taskAssetRepo.(service.AuditAssetFlowRepo); ok {
		auditV7Options = append(auditV7Options, service.WithAuditV7AssetFlowRepo(assetFlowRepo))
	}
	auditV7Svc := service.NewAuditV7Service(taskRepo, auditV7Repo, taskEventRepo, codeRuleSvc, mdb, auditV7Options...)
	outsourceSvc := service.NewOutsourceService(outsourceRepo, taskRepo, auditV7Repo, taskEventRepo, codeRuleSvc, mdb)
	taskEventSvc := service.NewTaskEventService(taskEventRepo, taskRepo,
		service.WithTaskEventUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	warehouseSvc := service.NewWarehouseService(taskRepo, taskAssetRepo, warehouseRepo, taskEventRepo, mdb,
		service.WithWarehouseDataScopeResolver(taskDataScopeResolver),
		service.WithWarehouseScopeUserRepo(userRepo),
		service.WithWarehouseCustomizationJobRepo(customizationJobRepo),
		service.WithWarehouseModuleSync(taskModuleRepo, taskModuleEventRepo),
		service.WithWarehouseFilingTrigger(taskSvc))
	operationLogSvc := service.NewOperationLogService(taskEventRepo, exportJobEventRepo, integrationCallLogRepo)
	notificationGen := notificationsvc.NewGenerator(notificationSvc, moduleNotificationRepo, logger.Named("notification_generator"))
	blueprintRules.SetNotificationGenerator(notificationGen)
	taskAssignmentSvc := service.NewTaskAssignmentService(taskRepo, taskEventRepo, mdb,
		service.WithTaskAssignmentDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssignmentScopeUserRepo(userRepo),
		service.WithTaskAssignmentNotificationService(notificationSvc),
		service.WithTaskAssignmentModuleSync(taskModuleRepo, taskModuleEventRepo))
	taskDraftSvc := taskdraftsvc.NewService(taskDraftRepo, permissionLogRepo, mdb)
	erpProductSvc := erpproductsvc.NewService(erpBridgeSvc)
	designSourceSvc := designsourcesvc.NewService(designSourceRepo)
	searchSvc := searchsvc.NewService(searchRepo)
	searchSvc.SetLogger(logger.Named("global_search"))
	searchSvc.SetExternalAssetSearchProvider(externalAssetSvc)
	predictionSvc := predictionsvc.NewService(predictionRepo)
	workflowTraceEventSvc := service.NewWorkflowTraceEventService(workflowTraceEventRepo)
	r3PoolQuerySvc := task_pool.NewPoolQueryService(mdb)
	r3ClaimSvc := task_pool.NewClaimService(taskRepo, taskModuleRepo, taskModuleEventRepo, mdb, task_pool.WithNotificationGenerator(notificationGen), task_pool.WithWebSocketHub(wsHub))
	r3ModuleSvc := r3module.NewActionService(taskRepo, taskModuleRepo, taskModuleEventRepo, referenceFileRefFlatRepo, mdb, blueprintRules, r3module.WithNotificationGenerator(notificationGen))
	r3CancelSvc := task_cancel.NewService(taskRepo, taskModuleRepo, taskModuleEventRepo, mdb)
	r3DetailSvc := task_aggregator.NewDetailService(taskRepo, taskModuleRepo, taskModuleEventRepo, referenceFileRefFlatRepo,
		task_aggregator.WithTaskAssetRepo(taskAssetRepo),
		task_aggregator.WithTaskRetouchRequirementRepo(taskRetouchRequirementRepo),
		task_aggregator.WithReferenceFileRefEnricher(service.NewReferenceFileRefsEnricher(ossDirectSvc, nil)),
		task_aggregator.WithUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	aiSummaryClient := aiagentsvc.NewAnthropicCompatibleClient(aiagentsvc.Config{
		Enabled:         cfg.AI.Enabled,
		Provider:        cfg.AI.Provider,
		BaseURL:         cfg.AI.BaseURL,
		APIKey:          cfg.AI.APIKey,
		Model:           cfg.AI.Model,
		Timeout:         cfg.AI.Timeout,
		MaxTokens:       cfg.AI.MaxTokens,
		RateLimitWindow: cfg.AI.RateLimitWindow,
		RateLimitMax:    cfg.AI.RateLimitMax,
		RateLimiter:     aiagentsvc.NewRedisAIRateLimiter(rdb, "omp"),
	}, logger.Named("ai_agent"))
	trendProviders, expectedTrendSources := reportl1svc.NewDefaultTrendProviders(reportl1svc.TrendProviderConfig{
		ChinaHotURL:         cfg.BusinessTrend.ChinaHotURL,
		ApifyToken:          cfg.BusinessTrend.ApifyToken,
		ApifyBaseURL:        cfg.BusinessTrend.ApifyBaseURL,
		ApifyDouyinHotActor: cfg.BusinessTrend.ApifyDouyinHotActor,
		ApifyDouyinActor:    cfg.BusinessTrend.ApifyDouyinActor,
		ApifyRedNoteActor:   cfg.BusinessTrend.ApifyRedNoteActor,
		Apify1688Actor:      cfg.BusinessTrend.Apify1688Actor,
		ApifyTaobaoActor:    cfg.BusinessTrend.ApifyTaobaoActor,
		Timeout:             cfg.BusinessTrend.Timeout,
		MaxExternalKeywords: cfg.BusinessTrend.MaxExternalKeywords,
		MaxExternalItems:    cfg.BusinessTrend.MaxExternalItems,
	}, logger.Named("business_trends"))
	reportL1Svc := reportl1svc.NewService(reportL1Repo,
		reportl1svc.WithPermissionLogRepo(permissionLogRepo),
		reportl1svc.WithReportL1Redis(rdb),
		reportl1svc.WithKPIAnalysisRepo(kpiAnalysisRepo),
		reportl1svc.WithKPIAnalysisGenerator(aiSummaryClient),
		reportl1svc.WithBusinessTrendRepo(businessTrendRepo),
		reportl1svc.WithBusinessTrendGenerator(aiSummaryClient),
		reportl1svc.WithBusinessTrendProviders(trendProviders, expectedTrendSources))
	taskAISummarySvc := taskaisummarysvc.NewService(r3DetailSvc, taskEventSvc, taskCostOverrideEventRepo, aiSummaryClient)
	assetWorkbenchOptions := []assetworkbench.Option{
		assetworkbench.WithRepository(assetWorkbenchRepo, mdb),
		assetworkbench.WithUserRepository(userRepo),
		assetworkbench.WithIdentityRegistrar(identitySvc),
		assetworkbench.WithNotificationCreator(notificationSvc),
		assetworkbench.WithOSSDirect(ossDirectSvc),
		assetworkbench.WithPreviewRenderer(service.NewExternalAssetPreviewRenderer()),
		assetworkbench.WithSystemAssetSearcher(globalAssetCenterSvc),
	}
	if sessionRevoker, ok := userSessionRepo.(assetworkbench.UserSessionRevoker); ok {
		assetWorkbenchOptions = append(assetWorkbenchOptions, assetworkbench.WithUserSessionRepository(sessionRevoker))
	}
	assetWorkbenchSvc := assetworkbench.NewService(assetworkbench.Config{
		Timezone:                 cfg.AssetWorkbench.Timezone,
		OSSPrefix:                cfg.AssetWorkbench.OSSPrefix,
		UploadSessionTTL:         cfg.AssetWorkbench.UploadSessionTTL,
		PreviewWorkerLeaseTTL:    cfg.AssetWorkbench.PreviewWorkerLeaseTTL,
		PreviewWorkerMaxAttempts: cfg.AssetWorkbench.PreviewWorkerMaxAttempts,
		BatchJobWorkerLeaseTTL:   cfg.AssetWorkbench.BatchJobWorkerLeaseTTL,
	}, assetWorkbenchOptions...)

	skuH := handler.NewSKUHandler(skuSvc)
	auditH := handler.NewAuditHandler(auditSvc)
	agentH := handler.NewAgentHandler(agentSvc)
	incidentH := handler.NewIncidentHandler(incidentSvc)
	policyH := handler.NewPolicyHandler(policySvc)
	authH := handler.NewAuthHandler(identitySvc, cfg.AssetWorkbench.CookieDomain)
	routeAccessCatalog := transport.NewRouteAccessCatalog()
	userAdminH := handler.NewUserAdminHandler(identitySvc, routeAccessCatalog, operationLogSvc, workflowTraceEventSvc)

	// V7 handlers
	erpBridgeH := handler.NewERPBridgeHandler(erpBridgeSvc)
	productH := handler.NewProductHandler(productSvc)
	productManagementH := handler.NewProductManagementHandler(productManagementSvc, costRecalculationSvc)
	categoryH := handler.NewCategoryHandler(categorySvc)
	categoryMappingH := handler.NewCategoryERPMappingHandler(categoryMappingSvc)
	costRuleH := handler.NewCostRuleHandler(costRuleSvc)
	costRuleBindingH := handler.NewCostRuleBindingHandler(costRuleBindingSvc)
	erpSyncH := handler.NewERPSyncHandler(erpSyncSvc)
	taskH := handler.NewTaskHandler(taskSvc, costRuleSvc, taskDetailSvc)
	taskH.SetR3Services(r3PoolQuerySvc, r3ClaimSvc, r3ModuleSvc, r3CancelSvc)
	taskAssignmentH := handler.NewTaskAssignmentHandler(taskAssignmentSvc)
	taskAssetH := handler.NewTaskAssetHandler(taskAssetSvc)
	taskAssetCenterH := handler.NewTaskAssetCenterHandler(taskAssetCenterSvc)
	taskAssetCenterH.SetGlobalAssetServices(globalAssetCenterSvc, globalAssetLifecycleSvc)
	taskCreateReferenceUploadH := handler.NewTaskCreateReferenceUploadHandler(taskCreateReferenceUploadSvc)
	assetUploadH := handler.NewAssetUploadHandler(assetUploadSvc)
	assetFilesH := handler.NewAssetFilesHandler(cfg.UploadService.BaseURL, cfg.UploadService.InternalToken, cfg.UploadService.StorageProvider, logger, ossDirectSvc)
	assetFilesH.SetERPImageProxy(taskAssetRepo, erpImageProxySigner)
	assetFilesTaskAssetRepo, ok := taskAssetRepo.(handler.AssetFilesTaskAssetRepo)
	if !ok {
		logger.Fatal("task asset repo does not support asset file access checks")
	}
	assetFilesStorageRefRepo, ok := assetStorageRefRepo.(handler.AssetFilesStorageRefRepo)
	if !ok {
		logger.Fatal("asset storage ref repo does not support asset file access checks")
	}
	assetFilesH.SetFileAccessPolicy(taskRepo, assetFilesTaskAssetRepo, assetFilesStorageRefRepo, userRepo)
	designSubmissionH := handler.NewDesignSubmissionHandler(taskAssetSvc, taskAssetCenterSvc, taskSvc)
	taskDetailH := handler.NewTaskDetailHandler(r3DetailSvc)
	taskAISummaryH := handler.NewTaskAISummaryHandler(taskAISummarySvc)
	taskCostOverrideH := handler.NewTaskCostOverrideHandler(taskCostOverrideSvc)
	taskBoardH := handler.NewTaskBoardHandler(taskBoardSvc)
	taskBatchExcelH := handler.NewTaskBatchExcelHandler(taskBatchTemplateSvc, taskBatchParseSvc)
	taskSingleExcelH := handler.NewTaskSingleExcelHandler(taskSingleTemplateSvc, taskSingleParseSvc)
	workbenchH := handler.NewWorkbenchHandler(workbenchSvc)
	assetWorkbenchH := handler.NewAssetWorkbenchHandler(assetWorkbenchSvc, cfg.AssetWorkbench.CookieDomain)
	exportCenterH := handler.NewExportCenterHandler(exportCenterSvc)
	integrationCenterH := handler.NewIntegrationCenterHandler(integrationCenterSvc)
	integrationCenterH.SetExternalAssetEventService(externalAssetSvc)
	codeRuleH := handler.NewCodeRuleHandler(codeRuleSvc)
	ruleTemplateSvc := service.NewRuleTemplateService(ruleTemplateRepo)
	ruleTemplateH := handler.NewRuleTemplateHandler(ruleTemplateSvc)
	auditV7H := handler.NewAuditV7Handler(auditV7Svc, taskEventSvc)
	auditLogH := handler.NewAuditLogHandler(auditV7Repo, taskRepo, userRepo)
	outsourceH := handler.NewOutsourceHandler(outsourceSvc)
	warehouseH := handler.NewWarehouseHandler(warehouseSvc)
	jstUserImportSvc := service.NewJSTUserImportService(erpBridgeSvc, userRepo, mdb, cfg.Auth)
	jstUserAdminH := handler.NewJSTUserAdminHandler(erpBridgeSvc, jstUserImportSvc)
	serverLogSvc := service.NewServerLogService(serverLogRepo)
	serverLogH := handler.NewServerLogHandler(serverLogSvc)
	orgMoveH := handler.NewOrgMoveRequestHandler(orgMoveSvc)
	taskDraftH := handler.NewTaskDraftHandler(taskDraftSvc)
	notificationH := handler.NewNotificationHandler(notificationSvc)
	erpProductH := handler.NewERPProductHandler(erpProductSvc)
	designSourceH := handler.NewDesignSourceHandler(designSourceSvc)
	searchH := handler.NewSearchHandler(searchSvc)
	reportL1H := handler.NewReportL1Handler(reportL1Svc, permissionLogRepo)
	experienceH := handler.NewExperienceHandler(experienceSvc)
	predictionH := handler.NewPredictionHandler(predictionSvc)
	predictionH.SetExperienceService(experienceSvc)
	wsH := transportws.NewHandler(identitySvc, wsHub)

	// ── 6. HTTP router ────────────────────────────────────────────────────────
	router := transport.NewRouter(skuH, auditH, agentH, incidentH, policyH, authH, userAdminH, erpBridgeH, productH, productManagementH, categoryH, categoryMappingH, costRuleH, costRuleBindingH, erpSyncH, taskH, taskAssignmentH, taskAssetH, taskAssetCenterH, taskCreateReferenceUploadH, assetUploadH, assetFilesH, designSubmissionH, taskDetailH, taskAISummaryH, taskCostOverrideH, taskBoardH, taskBatchExcelH, taskSingleExcelH, workbenchH, assetWorkbenchH, exportCenterH, integrationCenterH, codeRuleH, ruleTemplateH, auditV7H, auditLogH, outsourceH, warehouseH, jstUserAdminH, serverLogH, orgMoveH, taskDraftH, notificationH, erpProductH, designSourceH, searchH, reportL1H, experienceH, predictionH, wsH, routeAccessCatalog, identitySvc, identitySvc, logger, workflowTraceEventSvc)

	// ── 7. Background workers ─────────────────────────────────────────────────
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	workers.NewGroup(workers.GroupDeps{
		DB:                              db,
		Redis:                           rdb,
		Logger:                          logger,
		ERPSync:                         erpSyncSvc,
		ProductManagement:               productManagementSvc,
		SKUComboSync:                    skuComboSyncSvc,
		Notification:                    notificationSvc,
		AssetWorkbenchPreview:           assetWorkbenchSvc,
		AssetWorkbenchMaintenance:       assetWorkbenchSvc,
		ERPEnabled:                      cfg.ERP.Enabled,
		ERPInterval:                     cfg.ERP.Interval,
		WebPushEnabled:                  cfg.WebPush.Enabled,
		WebPushInterval:                 cfg.WebPush.WorkerInterval,
		WebPushLimit:                    cfg.WebPush.WorkerLimit,
		SKUSyncFailureReconcileInterval: cfg.WebPush.SKUSyncFailureScanInterval,
		SKUSyncFailureReconcileLimit:    cfg.WebPush.SKUSyncFailureScanLimit,
		AssetWorkbenchPreviewEnabled:    cfg.AssetWorkbench.PreviewWorkerEnabled,
		AssetWorkbenchPreviewInterval:   cfg.AssetWorkbench.PreviewWorkerInterval,
		AssetWorkbenchPreviewLimit:      cfg.AssetWorkbench.PreviewWorkerLimit,
		AssetWorkbenchExpiryEnabled:     cfg.AssetWorkbench.UploadExpiryWorkerEnabled,
		AssetWorkbenchExpiryInterval:    cfg.AssetWorkbench.UploadExpiryWorkerInterval,
		AssetWorkbenchExpiryLimit:       cfg.AssetWorkbench.UploadExpiryWorkerLimit,
		AssetWorkbenchBatchJob:          assetWorkbenchSvc,
		AssetWorkbenchBatchJobEnabled:   cfg.AssetWorkbench.BatchJobWorkerEnabled,
		AssetWorkbenchBatchJobInterval:  cfg.AssetWorkbench.BatchJobWorkerInterval,
		AssetWorkbenchBatchJobLimit:     cfg.AssetWorkbench.BatchJobWorkerLimit,
	}).Start(workerCtx)
	startExperienceWorker(workerCtx, experienceSvc, cfg.Experience, logger.Named("experience_worker"))
	if wecomSender.Start(workerCtx) {
		logger.Info("wecom aibot sender started", zap.String("chat_id", cfg.WeCom.AiBotDefaultChatID))
	}
	startExternalAssetRefresh(workerCtx, externalAssetSvc, logger.Named("external_assets"))
	logger.Info("background workers started")

	// ── 7.1 Cron(R6.A.2) ─────────────────────────────────────────────────────
	cronInst := scheduler.New(workerCtx, log.New(os.Stderr, "", log.LstdFlags))
	if envFlag("ENABLE_CRON_OSS_365") {
		ossSpec := envOr("CRON_SCHEDULE_OSS_365", "0 3 * * *")
		cleanupJob := assetlifecycle.NewCleanupJob(taskAssetLifecycleRepo, mdb, ossDirectSvc, log.New(os.Stderr, "[ASSET-CLEANUP-CRON] ", log.LstdFlags))
		if err := cronInst.Add("oss-365", ossSpec, func(ctx context.Context) error {
			_, appErr := cleanupJob.Run(ctx, assetlifecycle.CleanupOptions{Limit: 1000})
			if appErr != nil {
				return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
			}
			return nil
		}); err != nil {
			logger.Fatal("cron oss-365 add failed", zap.Error(err))
		}
		logger.Info("cron oss-365 enabled", zap.String("spec", ossSpec))
	}
	if envFlag("ENABLE_CRON_DRAFTS_7D") {
		draftSpec := envOr("CRON_SCHEDULE_DRAFTS_7D", "0 4 * * *")
		if err := cronInst.Add("drafts-7d", draftSpec, func(ctx context.Context) error {
			_, err := taskDraftSvc.CleanupExpired(ctx)
			return err
		}); err != nil {
			logger.Fatal("cron drafts-7d add failed", zap.Error(err))
		}
		logger.Info("cron drafts-7d enabled", zap.String("spec", draftSpec))
	}
	if envFlag("ENABLE_CRON_AUTO_ARCHIVE") {
		archiveSpec := envOr("CRON_SCHEDULE_AUTO_ARCHIVE", "0 5 * * *")
		autoArchiveJob := tasklifecycle.NewAutoArchiveJob(taskAutoArchiveRepo, mdb, log.New(os.Stderr, "[TASK-AUTO-ARCHIVE-CRON] ", log.LstdFlags))
		if err := cronInst.Add("auto-archive", archiveSpec, func(ctx context.Context) error {
			_, appErr := autoArchiveJob.Run(ctx, tasklifecycle.AutoArchiveOptions{Limit: 1000, CutoffDays: 90})
			if appErr != nil {
				return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
			}
			return nil
		}); err != nil {
			logger.Fatal("cron auto-archive add failed", zap.Error(err))
		}
		logger.Info("cron auto-archive enabled", zap.String("spec", archiveSpec))
	}
	if envFlag("ENABLE_CRON_WAREHOUSE_AUTO_RELEASE") {
		releaseSpec := envOr("CRON_SCHEDULE_WAREHOUSE_AUTO_RELEASE", "*/5 * * * *")
		autoReleaseCandidateRepo, ok := taskAssetRepo.(service.WarehouseAutoReleaseCandidateRepo)
		if !ok {
			logger.Fatal("cron warehouse-auto-release requires candidate repo support")
		}
		autoReleaseJob := service.NewWarehouseAutoReleaseJob(
			autoReleaseCandidateRepo,
			taskRepo,
			warehouseRepo,
			taskEventRepo,
			mdb,
			log.New(os.Stderr, "[WAREHOUSE-AUTO-RELEASE-CRON] ", log.LstdFlags),
			service.WithWarehouseAutoReleaseModuleRepos(taskModuleRepo, taskModuleEventRepo),
			service.WithWarehouseAutoReleaseProductManagementCloseSyncer(productManagementSvc),
			service.WithWarehouseAutoReleaseNotificationService(notificationSvc),
		)
		if err := cronInst.Add("warehouse-auto-release", releaseSpec, func(ctx context.Context) error {
			result, appErr := autoReleaseJob.Run(ctx, service.WarehouseAutoReleaseOptions{
				Limit:         envInt("WAREHOUSE_AUTO_RELEASE_LIMIT", 100),
				GracePeriod:   time.Duration(envInt("WAREHOUSE_AUTO_RELEASE_GRACE_MINUTES", 30)) * time.Minute,
				SystemActorID: int64(envInt("WAREHOUSE_AUTO_RELEASE_SYSTEM_ACTOR_ID", 0)),
			})
			if appErr != nil {
				return fmt.Errorf("%s: %s", appErr.Code, appErr.Message)
			}
			logger.Info("cron warehouse-auto-release run",
				zap.Int("scanned", result.Scanned),
				zap.Int("released", result.Released),
				zap.Int("skipped", result.Skipped),
				zap.Time("cutoff", result.Cutoff),
			)
			return nil
		}); err != nil {
			logger.Fatal("cron warehouse-auto-release add failed", zap.Error(err))
		}
		logger.Info("cron warehouse-auto-release enabled", zap.String("spec", releaseSpec))
	}
	if envFlag("ENABLE_CRON_REPORT_L1_DAILY") {
		reportSpec := envOr("CRON_SCHEDULE_REPORT_L1_DAILY", "*/10 * * * *")
		if err := cronInst.Add("report-l1-daily", reportSpec, func(ctx context.Context) error {
			days := envInt("REPORT_L1_DAILY_REFRESH_DAYS", 3)
			if days < 1 {
				days = 1
			}
			to := time.Now().UTC().Truncate(24 * time.Hour)
			from := to.AddDate(0, 0, -(days - 1))
			if err := reportL1Repo.RefreshDailyAggregates(ctx, from, to); err != nil {
				return err
			}
			logger.Info("cron report-l1-daily run",
				zap.Time("from", from),
				zap.Time("to", to),
				zap.Int("days", days),
			)
			return nil
		}); err != nil {
			logger.Fatal("cron report-l1-daily add failed", zap.Error(err))
		}
		logger.Info("cron report-l1-daily enabled", zap.String("spec", reportSpec))
	}
	cronInst.Start()
	logger.Info("cron started", zap.Int("entries", len(cronInst.Entries())))

	// ── 8. HTTP server ────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	go func() {
		logger.Info("HTTP server listening", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", zap.Error(err))
		}
	}()

	// ── 9. Graceful shutdown ──────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received — draining...")

	cancelWorkers() // stop background workers first

	cronStopCtx, cronCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cronCancel()
	if err := cronInst.Stop(cronStopCtx); err != nil {
		logger.Warn("cron stop timeout/err", zap.Error(err))
	}
	logger.Info("cron stop")

	shutCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("HTTP shutdown error", zap.Error(err))
	}
	logger.Info("server stopped gracefully")
}

func buildLogger(level string) *zap.Logger {
	var cfg zap.Config
	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build logger: %v", err))
	}
	return logger
}

func startExternalAssetRefresh(ctx context.Context, svc *externalassets.Service, logger *zap.Logger) {
	if svc == nil || !svc.Enabled() {
		return
	}
	go func() {
		interval := svc.SyncInterval()
		fullSyncInterval := svc.FullSyncInterval()
		var lastFullSync time.Time
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		logger.Info("external asset refresh worker started", zap.Duration("interval", interval), zap.Duration("full_sync_interval", fullSyncInterval))
		runRefresh := func() {
			refreshCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()
			if svc.FullSyncReady() && (lastFullSync.IsZero() || time.Since(lastFullSync) >= fullSyncInterval) {
				lastFullSync = time.Now()
				full, err := svc.SyncFullIndex(refreshCtx)
				if err != nil {
					logger.Warn("external full index refresh finished with error", zap.Error(err))
				}
				if full != nil && (len(full.Mounts) > 0 || full.ScannedCount > 0 || full.UpsertedCount > 0) {
					logger.Info("external full index refresh finished",
						zap.Int("mounts", len(full.Mounts)),
						zap.Int("scanned", full.ScannedCount),
						zap.Int("upserted", full.UpsertedCount),
					)
				}
			}
			if svc.LegacyIndexRefreshReady() {
				keywords := []string{"jpg", "jpeg", "png", "webp", "psd", "psb", "ai", "pdf", "tif", "tiff", "2026", "2025"}
				seen := map[string]struct{}{}
				for _, keyword := range keywords {
					seen[keyword] = struct{}{}
				}
				for _, keyword := range svc.RecentKeywords(50) {
					if _, ok := seen[keyword]; ok {
						continue
					}
					seen[keyword] = struct{}{}
					keywords = append(keywords, keyword)
				}
				for _, keyword := range keywords {
					if err := svc.SyncKeyword(refreshCtx, keyword, 200); err != nil {
						logger.Warn("external keyword refresh failed", zap.String("keyword", keyword), zap.Error(err))
					}
				}
				ready, failed, err := svc.RefreshDirectURLs(refreshCtx, 100)
				if err != nil {
					logger.Warn("external direct url refresh failed", zap.Error(err))
				} else if ready > 0 || failed > 0 {
					logger.Info("external direct url refresh finished", zap.Int("ready", ready), zap.Int("failed", failed))
				}
			}
		}
		runRefresh()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runRefresh()
			}
		}
	}()
	startPrepareLoop := func(name string, wake <-chan struct{}, run func(context.Context, int) (int, error)) {
		go func() {
			interval := svc.PrepareInterval()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			logger.Info("external asset prepare worker started", zap.String("queue", name), zap.Duration("interval", interval))
			runPrepare := func() {
				prepareCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
				defer cancel()
				done, err := run(prepareCtx, svc.PrepareLimit())
				if err != nil {
					logger.Warn("external asset prepare failed", zap.String("queue", name), zap.Error(err))
				} else if done > 0 {
					logger.Info("external asset prepare finished", zap.String("queue", name), zap.Int("done", done))
				}
			}
			runPrepare()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					runPrepare()
				case <-wake:
					runPrepare()
				}
			}
		}()
	}
	startPrepareLoop("original", svc.OSSPrepareWake(), svc.ProcessPendingOSS)
	startPrepareLoop("preview", nil, svc.ProcessPendingPreview)
}

func startExperienceWorker(ctx context.Context, svc service.ExperienceService, cfg config.ExperienceConfig, logger *zap.Logger) {
	if svc == nil {
		return
	}
	interval := cfg.WorkerInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	batchSize := cfg.WorkerBatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		logger.Info("experience worker loop started", zap.Duration("interval", interval), zap.Int("batch_size", batchSize))
		runOnce := func() {
			if !svc.RuntimeFlags().WorkerEnabled {
				return
			}
			observerCtx, observerCancel := context.WithTimeout(ctx, interval)
			observerResult, appErr := svc.ProcessOutcomeObservers(observerCtx, batchSize)
			observerCancel()
			if appErr != nil {
				logger.Warn("experience outcome observer run failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
			}
			outboxCtx, outboxCancel := context.WithTimeout(ctx, interval)
			result, appErr := svc.ProcessOutbox(outboxCtx, batchSize)
			outboxCancel()
			if appErr != nil {
				logger.Warn("experience outbox worker run failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
			}
			attributionCtx, attributionCancel := context.WithTimeout(ctx, interval)
			attributionResult, appErr := svc.ProcessAttributions(attributionCtx, batchSize)
			attributionCancel()
			if appErr != nil {
				logger.Warn("experience attribution worker run failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
			}
			retentionCtx, retentionCancel := context.WithTimeout(ctx, interval)
			retentionResult, appErr := svc.ProcessRetention(retentionCtx, time.Now().UTC(), batchSize)
			retentionCancel()
			if appErr != nil {
				logger.Warn("experience retention run failed", zap.String("code", appErr.Code), zap.String("message", appErr.Message))
			}
			if observerResult.Scanned > 0 || observerResult.Failed > 0 || result.Claimed > 0 || result.Failed > 0 || result.DeadLetter > 0 ||
				attributionResult.Scanned > 0 || attributionResult.Failed > 0 ||
				retentionResult.BehaviorDeleted > 0 || retentionResult.RateLimitDeleted > 0 || retentionResult.ObservedTombstoned > 0 {
				logger.Info("experience worker run finished",
					zap.Int("observer_scanned", observerResult.Scanned),
					zap.Int("observer_baselines", observerResult.Baselines),
					zap.Int("observer_enqueued", observerResult.Enqueued),
					zap.Int("claimed", result.Claimed),
					zap.Int("processed", result.Processed),
					zap.Int("failed", result.Failed),
					zap.Int("dead_letter", result.DeadLetter),
					zap.Int("attribution_scanned", attributionResult.Scanned),
					zap.Int("attribution_created", attributionResult.Created),
					zap.Int("attribution_failed", attributionResult.Failed),
					zap.Int64("behavior_deleted", retentionResult.BehaviorDeleted),
					zap.Int64("rate_limit_deleted", retentionResult.RateLimitDeleted),
					zap.Int64("observed_tombstoned", retentionResult.ObservedTombstoned))
			}
		}
		runOnce()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce()
			}
		}
	}()
}

func erpRemoteServiceConfig(cfg *config.Config, log *zap.Logger) service.ERPRemoteClientConfig {
	return service.ERPRemoteClientConfig{
		BaseURL:                  cfg.ERPRemote.BaseURL,
		UpsertPath:               cfg.ERPRemote.UpsertPath,
		ItemStyleUpdatePath:      cfg.ERPRemote.ItemStyleUpdatePath,
		ShelveBatchPath:          cfg.ERPRemote.ShelveBatchPath,
		UnshelveBatchPath:        cfg.ERPRemote.UnshelveBatchPath,
		VirtualQtyPath:           cfg.ERPRemote.VirtualQtyPath,
		SyncLogsPath:             cfg.ERPRemote.SyncLogsPath,
		GetCompanyUsersPath:      cfg.ERPRemote.GetCompanyUsersPath,
		SkuQueryPath:             cfg.ERPRemote.SkuQueryPath,
		CombineSKUQueryPath:      cfg.ERPRemote.CombineSKUQueryPath,
		OrderActionQueryPath:     cfg.ERPRemote.OrderActionQueryPath,
		OpenWebCharset:           cfg.ERPRemote.OpenWebCharset,
		OpenWebVersion:           cfg.ERPRemote.OpenWebVersion,
		Timeout:                  cfg.ERPRemote.Timeout,
		RetryMax:                 cfg.ERPRemote.RetryMax,
		RetryBackoff:             cfg.ERPRemote.RetryBackoff,
		AuthMode:                 cfg.ERPRemote.AuthMode,
		AuthHeaderToken:          cfg.ERPRemote.AuthHeaderToken,
		AppKey:                   cfg.ERPRemote.AppKey,
		AppSecret:                cfg.ERPRemote.AppSecret,
		AccessToken:              cfg.ERPRemote.AccessToken,
		HeaderAppKey:             cfg.ERPRemote.HeaderAppKey,
		HeaderAccessToken:        cfg.ERPRemote.HeaderAccessToken,
		HeaderTimestamp:          cfg.ERPRemote.HeaderTimestamp,
		HeaderNonce:              cfg.ERPRemote.HeaderNonce,
		HeaderSignature:          cfg.ERPRemote.HeaderSignature,
		SignatureIncludeBodyHash: cfg.ERPRemote.SignatureIncludeBodyHash,
		Logger:                   log,
	}
}

func connectMySQL(cfg config.MySQLConfig) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	return db, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFlag(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func connectRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}

func sortedTaskOrgDepartmentKeys(departmentTeams map[string][]string) []string {
	keys := make([]string, 0, len(departmentTeams))
	for key := range departmentTeams {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
