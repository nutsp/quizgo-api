package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	accessrepo "virtual-exam-api/internal/accesslog/repository"
	accesshttp "virtual-exam-api/internal/accesslog/transport/http"
	accessuc "virtual-exam-api/internal/accesslog/usecase"
	dashboarduc "virtual-exam-api/internal/admin/dashboard/usecase"
	adminhttp "virtual-exam-api/internal/admin/transport/http"
	auditrepo "virtual-exam-api/internal/auditlog/repository"
	audithttp "virtual-exam-api/internal/auditlog/transport/http"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	oauthpkg "virtual-exam-api/internal/auth/oauth"
	oauthrepo "virtual-exam-api/internal/auth/oauth/repository"
	authhttp "virtual-exam-api/internal/auth/transport/http"
	authuc "virtual-exam-api/internal/auth/usecase"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/config"
	"virtual-exam-api/internal/database"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	enthttp "virtual-exam-api/internal/entitlement/transport/http"
	entuc "virtual-exam-api/internal/entitlement/usecase"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	attempthttp "virtual-exam-api/internal/examattempt/transport/http"
	attemptuc "virtual-exam-api/internal/examattempt/usecase"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	examsethttp "virtual-exam-api/internal/examset/transport/http"
	examsetuc "virtual-exam-api/internal/examset/usecase"
	esqrepo "virtual-exam-api/internal/examsetquestion/repository"
	esqhttp "virtual-exam-api/internal/examsetquestion/transport/http"
	esquc "virtual-exam-api/internal/examsetquestion/usecase"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	trackhttp "virtual-exam-api/internal/examtrack/transport/http"
	trackadminuc "virtual-exam-api/internal/examtrack/usecase"
	trackuc "virtual-exam-api/internal/examtrack/usecase"
	homehttp "virtual-exam-api/internal/home/transport/http"
	homeuc "virtual-exam-api/internal/home/usecase"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardhttp "virtual-exam-api/internal/leaderboard/transport/http"
	leaderboarduc "virtual-exam-api/internal/leaderboard/usecase"
	"virtual-exam-api/internal/media/storage"
	mediahttp "virtual-exam-api/internal/media/transport/http"
	"virtual-exam-api/internal/middleware"
	profilehttp "virtual-exam-api/internal/profile/transport/http"
	profileuc "virtual-exam-api/internal/profile/usecase"
	questionrepo "virtual-exam-api/internal/question/repository"
	questionuc "virtual-exam-api/internal/question/usecase"
	importrepo "virtual-exam-api/internal/questionimport/repository"
	importhttp "virtual-exam-api/internal/questionimport/transport/http"
	importuc "virtual-exam-api/internal/questionimport/usecase"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
	taghttp "virtual-exam-api/internal/questiontag/transport/http"
	taguc "virtual-exam-api/internal/questiontag/usecase"
	redisclient "virtual-exam-api/internal/redis"
	resultrepo "virtual-exam-api/internal/result/repository"
	resulthttp "virtual-exam-api/internal/result/transport/http"
	resultuc "virtual-exam-api/internal/result/usecase"
	scoringuc "virtual-exam-api/internal/scoring/usecase"
	settingsrepo "virtual-exam-api/internal/settings/repository"
	settingsuc "virtual-exam-api/internal/settings/usecase"
	subjectrepo "virtual-exam-api/internal/subject/repository"
	subjectuc "virtual-exam-api/internal/subject/usecase"
	userrepo "virtual-exam-api/internal/user/repository"
	useradminrepo "virtual-exam-api/internal/useradmin/repository"
	useradminhttp "virtual-exam-api/internal/useradmin/transport/http"
	useradminuc "virtual-exam-api/internal/useradmin/usecase"
	"virtual-exam-api/seed"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := database.NewPostgres(cfg)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}

	if cfg.AutoMigrate {
		database.MustMigrate(db,
			&userrepo.UserModel{},
			&oauthrepo.OAuthAccountModel{},
			&accessrepo.AccessLogModel{},
			&auditrepo.AuditLogModel{},
			&trackrepo.ExamTrackModel{},
			&examsetrepo.ExamSetModel{},
			&examsetrepo.LifecycleEventModel{},
			&questionrepo.SubjectModel{},
			&questionrepo.QuestionModel{},
			&questionrepo.ChoiceModel{},
			&questionrepo.ExamSetQuestionModel{},
			&esqrepo.QuestionRuleModel{},
			&attemptrepo.ExamAttemptModel{},
			&attemptrepo.ExamAnswerModel{},
			&attemptrepo.ProjectionOutboxModel{},
			&leaderboardrepo.SeasonModel{},
			&leaderboardrepo.ExamSetStopEventModel{},
			&leaderboardrepo.SeasonExamSetModel{},
			&leaderboardrepo.ScoreModel{},
			&leaderboardrepo.EntryModel{},
			&leaderboardrepo.AwardModel{},
			&leaderboardrepo.ProjectionFailureModel{},
			&importrepo.ImportJobModel{},
			&importrepo.ImportRowModel{},
			&tagrepo.QuestionTagModel{},
			&tagrepo.QuestionTagMappingModel{},
			&entrepo.EntitlementModel{},
			&settingsrepo.SystemSettingModel{},
		)
		if err := examsetrepo.ReconcilePublicationState(db); err != nil {
			log.Fatalf("reconcile exam set publication state: %v", err)
		}
		if err := leaderboardrepo.ReconcileLifecycleSchema(db); err != nil {
			log.Fatalf("reconcile leaderboard lifecycle schema: %v", err)
		}
	}

	if cfg.AutoSeed {
		if err := seed.Run(context.Background(), db); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	rdb, err := redisclient.NewClients(cfg)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()

	contentCache := cache.NewRedisCache(rdb.Content, "content", cfg.RedisCacheEnabled)
	userCache := cache.NewRedisCache(rdb.User, "user", cfg.RedisCacheEnabled)
	resultCache := cache.NewRedisCache(rdb.Result, "result", cfg.RedisCacheEnabled)
	cacheInvalidator := cache.NewInvalidator(contentCache, userCache, resultCache)
	runtimeLocks := cache.NewRuntimeLocks(rdb.Runtime)

	userRepository := userrepo.NewPostgresRepository(db)
	trackRepository := trackrepo.NewPostgresRepository(db)
	examSetRepository := examsetrepo.NewPostgresRepository(db)
	examSetAdminRepo := examsetrepo.NewAdminRepository(db)
	questionRepository := questionrepo.NewPostgresRepository(db)
	attemptRepository := attemptrepo.NewPostgresRepository(db)
	attemptCache := attemptrepo.NewRedisRepository(rdb.Runtime)
	settingsRepository := settingsrepo.NewPostgresRepository(db)
	leaderboardRepository := leaderboardrepo.NewPostgresRepository(db)
	leaderboardProjector := leaderboarduc.NewProjector(leaderboardRepository)
	leaderboardDispatcher := leaderboarduc.NewOutboxDispatcher(attemptRepository, examSetAdminRepo, leaderboardProjector, leaderboarduc.OutboxDispatcherConfig{})

	authUseCase := authuc.NewAuthUseCase(userRepository, cfg)
	oauthRepository := oauthrepo.NewPostgresRepository(db)
	oauthService := oauthpkg.NewService(userRepository, oauthRepository, authUseCase, cfg)
	entitlementRepository := entrepo.NewPostgresRepository(db)
	entitlementUseCase := entuc.NewUseCaseWithAttempts(entitlementRepository, examSetRepository, userRepository, attemptRepository, userCache, cacheInvalidator)
	trackUseCase := trackuc.NewExamTrackUseCase(trackRepository, examSetRepository, contentCache)
	settingsUseCase := settingsuc.NewUseCase(settingsRepository, contentCache)
	examSetUseCase := examsetuc.NewExamSetUseCaseWithAttempts(examSetRepository, questionRepository, entitlementUseCase, attemptRepository, contentCache)
	scoringUseCase := scoringuc.NewScoringUseCase()
	attemptUseCase := attemptuc.NewExamAttemptUseCase(
		attemptRepository,
		attemptCache,
		examSetRepository,
		questionRepository,
		scoringUseCase,
		entitlementUseCase,
		resultCache,
		runtimeLocks,
		cacheInvalidator,
		settingsUseCase,
		leaderboardDispatcher,
	)
	homeUseCase := homeuc.NewHomeUseCase(trackRepository, examSetRepository, attemptRepository, entitlementUseCase, contentCache)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Recover())
	e.Use(echomw.Logger())
	e.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	authMiddleware := middleware.JWTAuth(authUseCase)
	optionalAuth := middleware.OptionalJWTAuth(authUseCase)

	api := e.Group("/api/v1")
	homehttp.NewHandler(homeUseCase).RegisterRoutes(api, optionalAuth)
	trackhttp.NewHandler(trackUseCase).RegisterRoutes(api)
	examsethttp.NewHandler(examSetUseCase, attemptUseCase).RegisterRoutes(api, authMiddleware, optionalAuth)
	attempthttp.NewHandler(attemptUseCase).RegisterRoutes(api, authMiddleware)
	resultRepository := resultrepo.NewPostgresRepository(db)
	resultUseCase := resultuc.NewResultUseCase(resultRepository)
	resulthttp.NewHandler(resultUseCase).RegisterRoutes(api, authMiddleware)

	leaderboardUseCase := leaderboarduc.NewLeaderboardUseCase(leaderboardRepository)
	leaderboardReadLimiter := leaderboardhttp.NewRedisReadLimiter(rdb.Runtime, log.Default())
	leaderboardhttp.NewHandler(leaderboardUseCase, leaderboardReadLimiter).RegisterRoutes(api, authMiddleware)

	profileUseCase := profileuc.NewProfileUseCase(userRepository, resultRepository)
	profilehttp.NewHandler(profileUseCase).RegisterRoutes(api, authMiddleware)

	trackAdminRepo := trackrepo.NewAdminRepository(db)
	subjectAdminRepo := subjectrepo.NewSubjectAdminRepository(db)
	tagAdminRepo := tagrepo.NewTagAdminRepository(db)
	questionAdminRepo := questionrepo.NewQuestionAdminRepository(db, tagAdminRepo)
	setQuestionAdminRepo := questionrepo.NewExamSetQuestionAdminRepository(db)

	trackAdminUC := trackadminuc.NewAdminUseCase(trackAdminRepo, trackRepository, cacheInvalidator)
	examSetAdminUC := examsetuc.NewAdminUseCase(examSetAdminRepo, examSetRepository, trackRepository, trackAdminRepo, setQuestionAdminRepo, cacheInvalidator, leaderboardProjector)
	subjectAdminUC := subjectuc.NewSubjectUseCase(subjectAdminRepo)
	tagAdminUC := taguc.NewTagUseCase(tagAdminRepo, subjectAdminRepo)
	questionAdminUC := questionuc.NewAdminUseCase(questionAdminRepo, setQuestionAdminRepo, subjectAdminRepo, tagAdminUC, examSetRepository, examSetAdminRepo, trackAdminRepo, cacheInvalidator)
	dashboardUC := dashboarduc.NewDashboardUseCase(db)

	examSetQuestionRepo := esqrepo.NewPostgresRepository(db)
	examSetQuestionUC := esquc.NewUseCase(examSetQuestionRepo, questionAdminRepo, examSetRepository, examSetAdminRepo, trackAdminRepo, tagAdminRepo, cacheInvalidator)
	examSetQuestionHandler := esqhttp.NewHandler(examSetQuestionUC)

	importRepository := importrepo.NewRepository(db)
	uploadStore, err := storage.NewLocalStorage(cfg.UploadDir, cfg.UploadURLPath)
	if err != nil {
		log.Fatalf("upload storage: %v", err)
	}
	importUseCase := importuc.NewUseCase(importRepository, subjectAdminRepo, questionAdminRepo, tagAdminRepo, uploadStore)

	accessLogRepo := accessrepo.NewPostgresRepository(db)
	accessLogger := accessuc.NewLogger(accessLogRepo)
	accessLogAdminUC := accessuc.NewAdminUseCase(accessLogRepo)

	auditLogRepo := auditrepo.NewPostgresRepository(db)
	auditLogger := audituc.NewLogger(auditLogRepo)
	auditLogAdminUC := audituc.NewAdminUseCase(auditLogRepo)

	userAdminRepo := useradminrepo.NewUserAdminRepository(db)
	userAdminUC := useradminuc.NewUseCase(userAdminRepo, entitlementRepository, accessLogRepo, auditLogRepo, auditLogger)

	adminRoute := api.Group("/admin", authMiddleware, middleware.AdminOnly())
	taghttp.NewHandler(tagAdminUC, auditLogger, userRepository).RegisterRoutes(adminRoute)
	accesshttp.NewHandler(accessLogAdminUC).RegisterRoutes(adminRoute)
	audithttp.NewHandler(auditLogAdminUC).RegisterRoutes(adminRoute)
	useradminhttp.NewHandler(userAdminUC, userRepository).RegisterRoutes(adminRoute)
	enthttp.NewHandler(entitlementUseCase, auditLogger, userRepository).RegisterRoutes(adminRoute)
	enthttp.NewHandler(entitlementUseCase, auditLogger, userRepository).RegisterUserRoutes(api, authMiddleware)

	authhttp.NewHandler(authUseCase, oauthService, accessLogger, userRepository).RegisterRoutes(api, authMiddleware)

	importhttp.NewHandler(importUseCase, auditLogger, userRepository).
		RegisterRoutes(api, authMiddleware, middleware.AdminOnly())
	adminhttp.NewHandler(dashboardUC, trackAdminUC, examSetAdminUC, subjectAdminUC, questionAdminUC, examSetQuestionHandler, settingsUseCase, auditLogger, userRepository).
		RegisterRoutes(api, authMiddleware, middleware.AdminOnly())
	mediahttp.NewMediaHandler(uploadStore).RegisterRoutes(adminRoute)

	e.Static(cfg.UploadURLPath, cfg.UploadDir)

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	go func() {
		addr := ":" + cfg.AppPort
		log.Printf("virtual-exam-api listening on %s", addr)
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	dispatcherCtx, stopDispatcher := context.WithCancel(context.Background())
	dispatcherDone := make(chan struct{})
	go func() {
		defer close(dispatcherDone)
		leaderboardDispatcher.Run(dispatcherCtx)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	stopDispatcher()
	select {
	case <-dispatcherDone:
	case <-time.After(5 * time.Second):
		log.Printf("leaderboard dispatcher shutdown timed out")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
