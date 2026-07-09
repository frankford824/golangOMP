// Deprecated: cmd/api is a compatibility-only entrypoint kept temporarily for
// v0.4 convergence safety. The canonical production MAIN entrypoint is
// ./cmd/server.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"workflow/config"
	"workflow/policy"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
	aiagentsvc "workflow/service/aiagent"
	assetcenter "workflow/service/asset_center"
	assetlifecycle "workflow/service/asset_lifecycle"
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
	fmt.Fprintln(os.Stderr, "warning: cmd/api is a deprecated compatibility entrypoint; use ./cmd/server for production MAIN")

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	logger := buildLogger(cfg.Log.Level)
	defer logger.Sync() //nolint:errcheck
	logger.Info("task org catalog bootstrap", zap.Int("department_team_count", len(cfg.Auth.DepartmentTeams)), zap.Strings("department_keys", sortedTaskOrgDepartmentKeys(cfg.Auth.DepartmentTeams)))
	service.ConfigureTaskOrgCatalog(cfg.Auth)

	db, err := connectMySQL(cfg.MySQL)
	if err != nil {
		logger.Fatal("MySQL connect failed", zap.Error(err))
	}
	defer db.Close()
	logger.Info("MySQL connected")

	rdb, err := connectRedis(cfg.Redis)
	if err != nil {
		logger.Fatal("Redis connect failed", zap.Error(err))
	}
	defer rdb.Close()
	logger.Info("Redis connected", zap.String("addr", cfg.Redis.Addr))

	mdb := mysqlrepo.New(db)
	skuRepo := mysqlrepo.NewSKURepo(mdb)
	eventRepo := mysqlrepo.NewEventRepo(mdb)
	assetVersionRepo := mysqlrepo.NewAssetVersionRepo(mdb)
	auditRepo := mysqlrepo.NewAuditRepo(mdb)
	jobRepo := mysqlrepo.NewJobRepo(mdb)
	incidentRepo := mysqlrepo.NewIncidentRepo(mdb)
	policyRepo := mysqlrepo.NewPolicyRepo(mdb)
	engine := policy.NewEngine()

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
	taskModuleRepo := mysqlrepo.NewTaskModuleRepo(mdb)
	taskModuleEventRepo := mysqlrepo.NewTaskModuleEventRepo(mdb)
	referenceFileRefFlatRepo := mysqlrepo.NewReferenceFileRefFlatRepo(mdb)
	taskRetouchRequirementRepo := mysqlrepo.NewTaskRetouchRequirementRepo(mdb)
	taskReferenceAssetBindingRepo := mysqlrepo.NewTaskReferenceAssetBindingRepo(mdb)
	taskAssetSearchRepo := mysqlrepo.NewTaskAssetSearchRepo(mdb)
	taskAssetLifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mdb)
	externalAssetRepo := mysqlrepo.NewExternalAssetRepo(mdb)
	orgMoveRequestRepo := mysqlrepo.NewOrgMoveRequestRepo(mdb)
	taskDraftRepo := mysqlrepo.NewTaskDraftRepo(mdb)
	notificationRepo := mysqlrepo.NewNotificationRepo(mdb)
	designSourceRepo := mysqlrepo.NewDesignSourceRepo(mdb)
	moduleNotificationRepo := mysqlrepo.NewModuleNotificationRepo(mdb)
	searchRepo := mysqlrepo.NewSearchRepo(mdb)
	predictionRepo := mysqlrepo.NewPredictionRepo(mdb)
	reportL1Repo := mysqlrepo.NewReportL1Repo(mdb)
	kpiAnalysisRepo := mysqlrepo.NewKPIAnalysisRepo(mdb)
	businessTrendRepo := mysqlrepo.NewBusinessTrendRepo(mdb)
	workflowTraceEventRepo := mysqlrepo.NewWorkflowTraceEventRepo(mdb)
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

	codeRuleSvc := service.NewCodeRuleService(codeRuleRepo, mdb)
	blueprintRegistry := blueprint.NewRegistry()
	blueprintRules := blueprint.NewRuleEngine(blueprintRegistry, taskModuleRepo, taskModuleEventRepo, taskRepo)
	categorySvc := service.NewCategoryService(categoryRepo, mdb)
	categoryMappingSvc := service.NewCategoryERPMappingService(categoryERPMappingRepo, categoryRepo, mdb)
	costRuleSvc := service.NewCostRuleService(costRuleRepo, categoryRepo, mdb)
	costRuleBindingSvc := service.NewCostRuleBindingService(costRuleBindingRepo, costRuleRepo, mdb)
	productSvc := service.NewProductService(productRepo, categoryRepo, categoryERPMappingRepo)
	var erpBridgeClient service.ERPBridgeClient
	if service.ShouldUseLocalERPBridgeClient(cfg.Server.Port, cfg.ERPBridge.BaseURL) {
		erpBridgeClient = service.NewLocalERPBridgeClient(productRepo, categoryRepo, mdb, integrationCallLogRepo)
	} else {
		erpBridgeClient, err = service.NewERPBridgeClient(service.ERPBridgeClientConfig{
			BaseURL: cfg.ERPBridge.BaseURL,
			Timeout: cfg.ERPBridge.Timeout,
			Logger:  logger.Named("erp_bridge"),
		})
		if err != nil {
			logger.Fatal("ERP Bridge client config failed", zap.Error(err))
		}
	}
	erpBridgeSvc := service.NewERPBridgeService(erpBridgeClient, productRepo, mdb)
	erpProvider := service.NewStubERPProductProvider(cfg.ERP.StubFile)
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
	erpImageProxySigner := service.NewERPImageProxySigner(service.ERPImageProxyConfig{
		PublicBaseURL: cfg.ERPImageProxy.PublicBaseURL,
		SigningSecret: cfg.ERPImageProxy.SigningSecret,
		TokenTTL:      cfg.ERPImageProxy.TokenTTL,
	})
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
		service.WithProductManagementERPBridge(erpBridgeSvc),
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
	skuComboSyncSvc := service.NewSKUComboSyncService(erpBridgeSvc, skuComboRepo, mdb)
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
		service.WithUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskDataScopeResolver(taskDataScopeResolver),
		service.WithTaskScopeUserRepo(userRepo),
		service.WithTaskBlueprintRuleEngine(blueprintRules),
		service.WithTaskRetouchRequirementRepo(taskRetouchRequirementRepo),
		service.WithTaskProductManagementCloseSyncer(productManagementSvc),
		service.WithTaskNotificationService(notificationSvc))
	taskBoardSvc := service.NewTaskBoardService(taskSvc)
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
		service.WithTaskAssetCenterReferenceFileRefFlatRepo(referenceFileRefFlatRepo))
	globalAssetCenterSvc := assetcenter.NewService(taskAssetSearchRepo, ossDirectSvc, uploadClient)
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
	skuH := handler.NewSKUHandler(skuSvc)
	auditH := handler.NewAuditHandler(auditSvc)
	agentH := handler.NewAgentHandler(agentSvc)
	incidentH := handler.NewIncidentHandler(incidentSvc)
	policyH := handler.NewPolicyHandler(policySvc)
	authH := handler.NewAuthHandler(identitySvc, cfg.AssetWorkbench.CookieDomain)
	routeAccessCatalog := transport.NewRouteAccessCatalog()
	userAdminH := handler.NewUserAdminHandler(identitySvc, routeAccessCatalog, operationLogSvc, workflowTraceEventSvc)

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
	exportCenterH := handler.NewExportCenterHandler(exportCenterSvc)
	integrationCenterH := handler.NewIntegrationCenterHandler(integrationCenterSvc)
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

	router := transport.NewRouter(skuH, auditH, agentH, incidentH, policyH, authH, userAdminH, erpBridgeH, productH, productManagementH, categoryH, categoryMappingH, costRuleH, costRuleBindingH, erpSyncH, taskH, taskAssignmentH, taskAssetH, taskAssetCenterH, taskCreateReferenceUploadH, assetUploadH, assetFilesH, designSubmissionH, taskDetailH, taskAISummaryH, taskCostOverrideH, taskBoardH, taskBatchExcelH, taskSingleExcelH, workbenchH, nil, exportCenterH, integrationCenterH, codeRuleH, ruleTemplateH, auditV7H, auditLogH, outsourceH, warehouseH, jstUserAdminH, serverLogH, orgMoveH, taskDraftH, notificationH, erpProductH, designSourceH, searchH, reportL1H, experienceH, predictionH, wsH, routeAccessCatalog, identitySvc, identitySvc, logger, workflowTraceEventSvc)

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
		ERPEnabled:                      cfg.ERP.Enabled,
		ERPInterval:                     cfg.ERP.Interval,
		WebPushEnabled:                  cfg.WebPush.Enabled,
		WebPushInterval:                 cfg.WebPush.WorkerInterval,
		WebPushLimit:                    cfg.WebPush.WorkerLimit,
		SKUSyncFailureReconcileInterval: cfg.WebPush.SKUSyncFailureScanInterval,
		SKUSyncFailureReconcileLimit:    cfg.WebPush.SKUSyncFailureScanLimit,
	}).Start(workerCtx)
	startExperienceWorker(workerCtx, experienceSvc, cfg.Experience, logger.Named("experience_worker"))
	if wecomSender.Start(workerCtx) {
		logger.Info("wecom aibot sender started", zap.String("chat_id", cfg.WeCom.AiBotDefaultChatID))
	}
	logger.Info("background workers started")

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown signal received, draining")

	cancelWorkers()

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
