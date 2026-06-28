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
	authhttp "virtual-exam-api/internal/auth/transport/http"
	authuc "virtual-exam-api/internal/auth/usecase"
	oauthpkg "virtual-exam-api/internal/auth/oauth"
	oauthrepo "virtual-exam-api/internal/auth/oauth/repository"
	"virtual-exam-api/internal/config"
	"virtual-exam-api/internal/cache"
	"virtual-exam-api/internal/database"
	attempthttp "virtual-exam-api/internal/examattempt/transport/http"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	attemptuc "virtual-exam-api/internal/examattempt/usecase"
	entrepo "virtual-exam-api/internal/entitlement/repository"
	enthttp "virtual-exam-api/internal/entitlement/transport/http"
	entuc "virtual-exam-api/internal/entitlement/usecase"
	examsethttp "virtual-exam-api/internal/examset/transport/http"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	examsetuc "virtual-exam-api/internal/examset/usecase"
	trackhttp "virtual-exam-api/internal/examtrack/transport/http"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	trackuc "virtual-exam-api/internal/examtrack/usecase"
	homehttp "virtual-exam-api/internal/home/transport/http"
	homeuc "virtual-exam-api/internal/home/usecase"
	"virtual-exam-api/internal/middleware"
	questionrepo "virtual-exam-api/internal/question/repository"
	questionuc "virtual-exam-api/internal/question/usecase"
	subjectrepo "virtual-exam-api/internal/subject/repository"
	subjectuc "virtual-exam-api/internal/subject/usecase"
	adminhttp "virtual-exam-api/internal/admin/transport/http"
	dashboarduc "virtual-exam-api/internal/admin/dashboard/usecase"
	accessrepo "virtual-exam-api/internal/accesslog/repository"
	accesshttp "virtual-exam-api/internal/accesslog/transport/http"
	accessuc "virtual-exam-api/internal/accesslog/usecase"
	auditrepo "virtual-exam-api/internal/auditlog/repository"
	audithttp "virtual-exam-api/internal/auditlog/transport/http"
	audituc "virtual-exam-api/internal/auditlog/usecase"
	esqrepo "virtual-exam-api/internal/examsetquestion/repository"
	esquc "virtual-exam-api/internal/examsetquestion/usecase"
	esqhttp "virtual-exam-api/internal/examsetquestion/transport/http"
	trackadminuc "virtual-exam-api/internal/examtrack/usecase"
	importhttp "virtual-exam-api/internal/questionimport/transport/http"
	importrepo "virtual-exam-api/internal/questionimport/repository"
	importuc "virtual-exam-api/internal/questionimport/usecase"
	tagrepo "virtual-exam-api/internal/questiontag/repository"
	taguc "virtual-exam-api/internal/questiontag/usecase"
	taghttp "virtual-exam-api/internal/questiontag/transport/http"
	leaderboardrepo "virtual-exam-api/internal/leaderboard/repository"
	leaderboardhttp "virtual-exam-api/internal/leaderboard/transport/http"
	leaderboarduc "virtual-exam-api/internal/leaderboard/usecase"
	profilehttp "virtual-exam-api/internal/profile/transport/http"
	profileuc "virtual-exam-api/internal/profile/usecase"
	resultrepo "virtual-exam-api/internal/result/repository"
	resulthttp "virtual-exam-api/internal/result/transport/http"
	resultuc "virtual-exam-api/internal/result/usecase"
	redisclient "virtual-exam-api/internal/redis"
	scoringuc "virtual-exam-api/internal/scoring/usecase"
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
			&questionrepo.SubjectModel{},
			&questionrepo.QuestionModel{},
			&questionrepo.ChoiceModel{},
			&questionrepo.ExamSetQuestionModel{},
			&attemptrepo.ExamAttemptModel{},
			&attemptrepo.ExamAnswerModel{},
			&importrepo.ImportJobModel{},
			&importrepo.ImportRowModel{},
			&tagrepo.QuestionTagModel{},
			&tagrepo.QuestionTagMappingModel{},
			&entrepo.EntitlementModel{},
		)
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
	questionRepository := questionrepo.NewPostgresRepository(db)
	attemptRepository := attemptrepo.NewPostgresRepository(db)
	attemptCache := attemptrepo.NewRedisRepository(rdb.Runtime)

	authUseCase := authuc.NewAuthUseCase(userRepository, cfg)
	oauthRepository := oauthrepo.NewPostgresRepository(db)
	oauthService := oauthpkg.NewService(userRepository, oauthRepository, authUseCase, cfg)
	entitlementRepository := entrepo.NewPostgresRepository(db)
	entitlementUseCase := entuc.NewUseCaseWithAttempts(entitlementRepository, examSetRepository, userRepository, attemptRepository, userCache, cacheInvalidator)
	trackUseCase := trackuc.NewExamTrackUseCase(trackRepository, examSetRepository, contentCache)
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

	leaderboardRepository := leaderboardrepo.NewPostgresRepository(db)
	leaderboardUseCase := leaderboarduc.NewLeaderboardUseCase(leaderboardRepository)
	leaderboardhttp.NewHandler(leaderboardUseCase).RegisterRoutes(api, authMiddleware)

	profileUseCase := profileuc.NewProfileUseCase(userRepository, resultRepository)
	profilehttp.NewHandler(profileUseCase).RegisterRoutes(api, authMiddleware)

	trackAdminRepo := trackrepo.NewAdminRepository(db)
	examSetAdminRepo := examsetrepo.NewAdminRepository(db)
	subjectAdminRepo := subjectrepo.NewSubjectAdminRepository(db)
	tagAdminRepo := tagrepo.NewTagAdminRepository(db)
	questionAdminRepo := questionrepo.NewQuestionAdminRepository(db, tagAdminRepo)
	setQuestionAdminRepo := questionrepo.NewExamSetQuestionAdminRepository(db)

	trackAdminUC := trackadminuc.NewAdminUseCase(trackAdminRepo, trackRepository, cacheInvalidator)
	examSetAdminUC := examsetuc.NewAdminUseCase(examSetAdminRepo, examSetRepository, trackRepository, trackAdminRepo, setQuestionAdminRepo, cacheInvalidator)
	subjectAdminUC := subjectuc.NewSubjectUseCase(subjectAdminRepo)
	tagAdminUC := taguc.NewTagUseCase(tagAdminRepo)
	questionAdminUC := questionuc.NewAdminUseCase(questionAdminRepo, setQuestionAdminRepo, subjectAdminRepo, tagAdminUC, examSetRepository, examSetAdminRepo, trackAdminRepo)
	dashboardUC := dashboarduc.NewDashboardUseCase(db)

	examSetQuestionRepo := esqrepo.NewPostgresRepository(db)
	examSetQuestionUC := esquc.NewUseCase(examSetQuestionRepo, questionAdminRepo, examSetRepository, examSetAdminRepo, trackAdminRepo, cacheInvalidator)
	examSetQuestionHandler := esqhttp.NewHandler(examSetQuestionUC)

	importRepository := importrepo.NewRepository(db)
	importUseCase := importuc.NewUseCase(importRepository, subjectAdminRepo, questionAdminRepo, tagAdminRepo)

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
	adminhttp.NewHandler(dashboardUC, trackAdminUC, examSetAdminUC, subjectAdminUC, questionAdminUC, examSetQuestionHandler, auditLogger, userRepository).
		RegisterRoutes(api, authMiddleware, middleware.AdminOnly())

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

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
