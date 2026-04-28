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
	assetcenter "workflow/service/asset_center"
	assetlifecycle "workflow/service/asset_lifecycle"
	"workflow/service/asset_lifecycle/scheduler"
	"workflow/service/blueprint"
	designsourcesvc "workflow/service/design_source"
	erpproductsvc "workflow/service/erp_product"
	r3module "workflow/service/module_action"
	notificationsvc "workflow/service/notification"
	orgmovesvc "workflow/service/org_move_request"
	reportl1svc "workflow/service/report_l1"
	searchsvc "workflow/service/search"
	"workflow/service/task_aggregator"
	"workflow/service/task_cancel"
	taskdraftsvc "workflow/service/task_draft"
	tasklifecycle "workflow/service/task_lifecycle"
	"workflow/service/task_pool"
	wsservice "workflow/service/websocket"
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
	categoryRepo := mysqlrepo.NewCategoryRepo(mdb)
	categoryERPMappingRepo := mysqlrepo.NewCategoryERPMappingRepo(mdb)
	costRuleRepo := mysqlrepo.NewCostRuleRepo(mdb)
	erpSyncRunRepo := mysqlrepo.NewERPSyncRunRepo(mdb)
	taskRepo := mysqlrepo.NewTaskRepo(mdb)
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
	taskAssetSearchRepo := mysqlrepo.NewTaskAssetSearchRepo(mdb)
	taskAssetLifecycleRepo := mysqlrepo.NewTaskAssetLifecycleRepo(mdb)
	taskAutoArchiveRepo := mysqlrepo.NewTaskAutoArchiveRepo(mdb)
	orgMoveRequestRepo := mysqlrepo.NewOrgMoveRequestRepo(mdb)
	taskDraftRepo := mysqlrepo.NewTaskDraftRepo(mdb)
	notificationRepo := mysqlrepo.NewNotificationRepo(mdb)
	designSourceRepo := mysqlrepo.NewDesignSourceRepo(mdb)
	moduleNotificationRepo := mysqlrepo.NewModuleNotificationRepo(mdb)
	searchRepo := mysqlrepo.NewSearchRepo(mdb)
	reportL1Repo := mysqlrepo.NewReportL1Repo(mdb)

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
	ossDirectSvc := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         cfg.OSSDirect.Enabled,
		Endpoint:        cfg.OSSDirect.Endpoint,
		Bucket:          cfg.OSSDirect.Bucket,
		AccessKeyID:     cfg.OSSDirect.AccessKeyID,
		AccessKeySecret: cfg.OSSDirect.AccessKeySecret,
		PresignExpiry:   cfg.OSSDirect.PresignExpiry,
		PublicEndpoint:  cfg.OSSDirect.PublicEndpoint,
		PartSize:        cfg.OSSDirect.PartSize,
	})
	if ossDirectSvc.Enabled() {
		logger.Info("OSS direct presign service enabled",
			zap.String("bucket", cfg.OSSDirect.Bucket),
			zap.String("endpoint", cfg.OSSDirect.Endpoint))
	}
	taskSvc := service.NewTaskServiceWithCatalog(taskRepo, procurementRepo, taskAssetRepo, taskEventRepo, taskCostOverrideEventRepo, warehouseRepo, categoryRepo, costRuleRepo, codeRuleSvc, mdb,
		service.WithTaskCostOverridePlaceholderRepos(taskCostOverrideReviewRepo, taskCostFinanceFlagRepo),
		service.WithERPBridgeSelectionBinding(erpBridgeSvc),
		service.WithTaskERPBridgeFilingTrace(integrationCallLogRepo),
		service.WithTaskReferenceFileRefValidation(uploadRequestRepo, assetStorageRefRepo),
		service.WithTaskReferenceFileRefFlatRepo(referenceFileRefFlatRepo),
		service.WithTaskReferenceFileRefsOSSDirectService(ossDirectSvc),
		service.WithTaskDesignAssetReadModel(designAssetRepo),
		service.WithTaskProductCodeSequenceRepo(productCodeSeqRepo),
		service.WithTaskCustomizationJobRepo(customizationJobRepo),
		service.WithTaskCustomizationPricingRuleRepo(customizationPricingRuleRepo),
		service.WithUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskDataScopeResolver(taskDataScopeResolver),
		service.WithTaskScopeUserRepo(userRepo),
		service.WithTaskBlueprintRuleEngine(blueprintRules))
	taskBoardSvc := service.NewTaskBoardService(taskSvc)
	workbenchSvc := service.NewWorkbenchService(workbenchPreferenceRepo)
	exportCenterSvc := service.NewExportCenterService(exportJobRepo, exportJobDispatchRepo, exportJobAttemptRepo, exportJobEventRepo, mdb)
	integrationCenterSvc := service.NewIntegrationCenterService(integrationCallLogRepo, integrationExecutionRepo, mdb)
	taskAssignmentSvc := service.NewTaskAssignmentService(taskRepo, taskEventRepo, mdb,
		service.WithTaskAssignmentDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssignmentScopeUserRepo(userRepo))
	taskAssetSvc := service.NewTaskAssetService(taskRepo, taskAssetRepo, taskEventRepo, uploadRequestRepo, assetStorageRefRepo, mdb,
		service.WithTaskAssetDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssetScopeUserRepo(userRepo),
		service.WithTaskAssetUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	assetUploadSvc := service.NewAssetUploadService(taskRepo, uploadRequestRepo, mdb)
	uploadClient := service.NewUploadServiceClient(service.UploadServiceClientConfig{
		Enabled:                 cfg.UploadService.Enabled,
		BaseURL:                 cfg.UploadService.BaseURL,
		BrowserMultipartBaseURL: cfg.UploadService.BrowserMultipartBaseURL,
		BrowserDownloadBaseURL:  cfg.UploadService.BrowserDownloadBaseURL,
		Timeout:                 cfg.UploadService.Timeout,
		InternalToken:           cfg.UploadService.InternalToken,
		StorageProvider:         cfg.UploadService.StorageProvider,
	})
	taskCreateReferenceUploadSvc := service.NewTaskCreateReferenceUploadService(
		uploadRequestRepo,
		assetStorageRefRepo,
		mdb,
		uploadClient,
		service.WithTaskCreateReferenceOSSDirectService(ossDirectSvc),
	)
	taskAssetCenterSvc := service.NewTaskAssetCenterService(taskRepo, designAssetRepo, taskAssetRepo, uploadRequestRepo, assetStorageRefRepo, taskEventRepo, mdb, uploadClient,
		service.WithOSSDirectService(ossDirectSvc),
		service.WithTaskAssetCenterDataScopeResolver(taskDataScopeResolver),
		service.WithTaskAssetCenterScopeUserRepo(userRepo),
		service.WithTaskAssetCenterUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	globalAssetCenterSvc := assetcenter.NewService(taskAssetSearchRepo, ossDirectSvc, uploadClient)
	globalAssetLifecycleSvc := assetlifecycle.NewService(taskAssetSearchRepo, taskAssetLifecycleRepo, mdb, ossDirectSvc)
	taskDetailSvc := service.NewTaskDetailAggregateService(taskRepo, procurementRepo, productRepo, costRuleRepo, auditV7Repo, outsourceRepo, taskAssetRepo, warehouseRepo, taskEventRepo, taskCostOverrideEventRepo, taskCostOverrideReviewRepo, taskCostFinanceFlagRepo,
		service.WithTaskDetailScopeUserRepo(userRepo),
		service.WithTaskDetailUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)),
		service.WithTaskDetailDesignAssetReadModel(designAssetRepo))
	taskCostOverrideSvc := service.NewTaskCostOverrideAuditService(taskRepo, taskCostOverrideEventRepo, taskEventRepo, taskCostOverrideReviewRepo, taskCostFinanceFlagRepo)
	auditV7Svc := service.NewAuditV7Service(taskRepo, auditV7Repo, taskEventRepo, codeRuleSvc, mdb,
		service.WithAuditV7DataScopeResolver(taskDataScopeResolver),
		service.WithAuditV7ScopeUserRepo(userRepo),
		service.WithAuditV7FilingTrigger(taskSvc))
	outsourceSvc := service.NewOutsourceService(outsourceRepo, taskRepo, auditV7Repo, taskEventRepo, codeRuleSvc, mdb)
	taskEventSvc := service.NewTaskEventService(taskEventRepo, taskRepo,
		service.WithTaskEventUserDisplayNameResolver(service.NewUserRepoDisplayNameResolver(userRepo)))
	warehouseSvc := service.NewWarehouseService(taskRepo, taskAssetRepo, warehouseRepo, taskEventRepo, mdb,
		service.WithWarehouseDataScopeResolver(taskDataScopeResolver),
		service.WithWarehouseScopeUserRepo(userRepo),
		service.WithWarehouseCustomizationJobRepo(customizationJobRepo),
		service.WithWarehouseFilingTrigger(taskSvc))
	operationLogSvc := service.NewOperationLogService(taskEventRepo, exportJobEventRepo, integrationCallLogRepo)
	wsHub := wsservice.NewHub(logger.Named("websocket"))
	notificationSvc := notificationsvc.NewService(notificationRepo, permissionLogRepo, wsHub, logger.Named("notification"))
	notificationGen := notificationsvc.NewGenerator(notificationSvc, moduleNotificationRepo, logger.Named("notification_generator"))
	taskDraftSvc := taskdraftsvc.NewService(taskDraftRepo, permissionLogRepo, mdb)
	erpProductSvc := erpproductsvc.NewService(erpBridgeSvc)
	designSourceSvc := designsourcesvc.NewService(designSourceRepo)
	searchSvc := searchsvc.NewService(searchRepo)
	reportL1Svc := reportl1svc.NewService(reportL1Repo, reportl1svc.WithPermissionLogRepo(permissionLogRepo))
	r3PoolQuerySvc := task_pool.NewPoolQueryService(mdb)
	r3ClaimSvc := task_pool.NewClaimService(taskRepo, taskModuleRepo, taskModuleEventRepo, mdb, task_pool.WithNotificationGenerator(notificationGen), task_pool.WithWebSocketHub(wsHub))
	r3ModuleSvc := r3module.NewActionService(taskRepo, taskModuleRepo, taskModuleEventRepo, referenceFileRefFlatRepo, mdb, blueprintRules, r3module.WithNotificationGenerator(notificationGen))
	r3CancelSvc := task_cancel.NewService(taskRepo, taskModuleRepo, taskModuleEventRepo, mdb)
	r3DetailSvc := task_aggregator.NewDetailService(taskRepo, taskModuleRepo, taskModuleEventRepo, referenceFileRefFlatRepo,
		task_aggregator.WithReferenceFileRefEnricher(service.NewReferenceFileRefsEnricher(ossDirectSvc, nil)))

	skuH := handler.NewSKUHandler(skuSvc)
	auditH := handler.NewAuditHandler(auditSvc)
	agentH := handler.NewAgentHandler(agentSvc)
	incidentH := handler.NewIncidentHandler(incidentSvc)
	policyH := handler.NewPolicyHandler(policySvc)
	authH := handler.NewAuthHandler(identitySvc)
	routeAccessCatalog := transport.NewRouteAccessCatalog()
	userAdminH := handler.NewUserAdminHandler(identitySvc, routeAccessCatalog, operationLogSvc)

	// V7 handlers
	erpBridgeH := handler.NewERPBridgeHandler(erpBridgeSvc)
	productH := handler.NewProductHandler(productSvc)
	categoryH := handler.NewCategoryHandler(categorySvc)
	categoryMappingH := handler.NewCategoryERPMappingHandler(categoryMappingSvc)
	costRuleH := handler.NewCostRuleHandler(costRuleSvc)
	erpSyncH := handler.NewERPSyncHandler(erpSyncSvc)
	taskH := handler.NewTaskHandler(taskSvc, costRuleSvc, taskDetailSvc)
	taskH.SetR3Services(r3PoolQuerySvc, r3ClaimSvc, r3ModuleSvc, r3CancelSvc)
	taskAssignmentH := handler.NewTaskAssignmentHandler(taskAssignmentSvc)
	taskAssetH := handler.NewTaskAssetHandler(taskAssetSvc)
	taskAssetCenterH := handler.NewTaskAssetCenterHandler(taskAssetCenterSvc)
	taskAssetCenterH.SetGlobalAssetServices(globalAssetCenterSvc, globalAssetLifecycleSvc)
	taskCreateReferenceUploadH := handler.NewTaskCreateReferenceUploadHandler(taskCreateReferenceUploadSvc)
	assetUploadH := handler.NewAssetUploadHandler(assetUploadSvc)
	assetFilesH := handler.NewAssetFilesHandler(cfg.UploadService.BaseURL, cfg.UploadService.InternalToken, cfg.UploadService.StorageProvider, logger)
	designSubmissionH := handler.NewDesignSubmissionHandler(taskAssetSvc, taskAssetCenterSvc, taskSvc)
	taskDetailH := handler.NewTaskDetailHandler(r3DetailSvc)
	taskCostOverrideH := handler.NewTaskCostOverrideHandler(taskCostOverrideSvc)
	taskBoardH := handler.NewTaskBoardHandler(taskBoardSvc)
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
	wsH := transportws.NewHandler(identitySvc, wsHub)

	// ── 6. HTTP router ────────────────────────────────────────────────────────
	router := transport.NewRouter(skuH, auditH, agentH, incidentH, policyH, authH, userAdminH, erpBridgeH, productH, categoryH, categoryMappingH, costRuleH, erpSyncH, taskH, taskAssignmentH, taskAssetH, taskAssetCenterH, taskCreateReferenceUploadH, assetUploadH, assetFilesH, designSubmissionH, taskDetailH, taskCostOverrideH, taskBoardH, workbenchH, exportCenterH, integrationCenterH, codeRuleH, ruleTemplateH, auditV7H, auditLogH, outsourceH, warehouseH, jstUserAdminH, serverLogH, orgMoveH, taskDraftH, notificationH, erpProductH, designSourceH, searchH, reportL1H, wsH, routeAccessCatalog, identitySvc, identitySvc, logger)

	// ── 7. Background workers ─────────────────────────────────────────────────
	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()
	workers.NewGroup(db, rdb, logger, erpSyncSvc, cfg.ERP.Enabled, cfg.ERP.Interval).Start(workerCtx)
	logger.Info("background workers started")

	// ── 7.1 Cron(R6.A.2) ─────────────────────────────────────────────────────
	cronInst := scheduler.New(workerCtx, log.New(os.Stderr, "", log.LstdFlags))
	if os.Getenv("ENABLE_CRON_OSS_365") == "1" {
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
	if os.Getenv("ENABLE_CRON_DRAFTS_7D") == "1" {
		draftSpec := envOr("CRON_SCHEDULE_DRAFTS_7D", "0 4 * * *")
		if err := cronInst.Add("drafts-7d", draftSpec, func(ctx context.Context) error {
			_, err := taskDraftSvc.CleanupExpired(ctx)
			return err
		}); err != nil {
			logger.Fatal("cron drafts-7d add failed", zap.Error(err))
		}
		logger.Info("cron drafts-7d enabled", zap.String("spec", draftSpec))
	}
	if os.Getenv("ENABLE_CRON_AUTO_ARCHIVE") == "1" {
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
