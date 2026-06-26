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
	"virtual-exam-api/internal/config"
	"virtual-exam-api/internal/database"
	attempthttp "virtual-exam-api/internal/examattempt/transport/http"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	attemptuc "virtual-exam-api/internal/examattempt/usecase"
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
	esqrepo "virtual-exam-api/internal/examsetquestion/repository"
	esquc "virtual-exam-api/internal/examsetquestion/usecase"
	esqhttp "virtual-exam-api/internal/examsetquestion/transport/http"
	trackadminuc "virtual-exam-api/internal/examtrack/usecase"
	importhttp "virtual-exam-api/internal/questionimport/transport/http"
	importrepo "virtual-exam-api/internal/questionimport/repository"
	importuc "virtual-exam-api/internal/questionimport/usecase"
	resultrepo "virtual-exam-api/internal/result/repository"
	resulthttp "virtual-exam-api/internal/result/transport/http"
	resultuc "virtual-exam-api/internal/result/usecase"
	redisclient "virtual-exam-api/internal/redis"
	scoringuc "virtual-exam-api/internal/scoring/usecase"
	userrepo "virtual-exam-api/internal/user/repository"
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
		)
	}

	if cfg.AutoSeed {
		if err := seed.Run(context.Background(), db); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	rdb, err := redisclient.NewClient(cfg)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()

	userRepository := userrepo.NewPostgresRepository(db)
	trackRepository := trackrepo.NewPostgresRepository(db)
	examSetRepository := examsetrepo.NewPostgresRepository(db)
	questionRepository := questionrepo.NewPostgresRepository(db)
	attemptRepository := attemptrepo.NewPostgresRepository(db)
	attemptCache := attemptrepo.NewRedisRepository(rdb.Raw())

	authUseCase := authuc.NewAuthUseCase(userRepository, cfg)
	trackUseCase := trackuc.NewExamTrackUseCase(trackRepository, examSetRepository)
	examSetUseCase := examsetuc.NewExamSetUseCase(examSetRepository, questionRepository)
	scoringUseCase := scoringuc.NewScoringUseCase()
	attemptUseCase := attemptuc.NewExamAttemptUseCase(
		attemptRepository,
		attemptCache,
		examSetRepository,
		questionRepository,
		scoringUseCase,
	)
	homeUseCase := homeuc.NewHomeUseCase(trackRepository, examSetRepository, attemptRepository)

	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Recover())
	e.Use(echomw.Logger())
	e.Use(middleware.CORS(cfg.CORSAllowedOrigins))

	authMiddleware := middleware.JWTAuth(authUseCase)
	optionalAuth := middleware.OptionalJWTAuth(authUseCase)

	api := e.Group("/api/v1")
	authhttp.NewHandler(authUseCase).RegisterRoutes(api, authMiddleware)
	homehttp.NewHandler(homeUseCase).RegisterRoutes(api, optionalAuth)
	trackhttp.NewHandler(trackUseCase).RegisterRoutes(api)
	examsethttp.NewHandler(examSetUseCase, attemptUseCase).RegisterRoutes(api, authMiddleware)
	attempthttp.NewHandler(attemptUseCase).RegisterRoutes(api, authMiddleware)
	resultRepository := resultrepo.NewPostgresRepository(db)
	resultUseCase := resultuc.NewResultUseCase(resultRepository)
	resulthttp.NewHandler(resultUseCase).RegisterRoutes(api, authMiddleware)

	trackAdminRepo := trackrepo.NewAdminRepository(db)
	examSetAdminRepo := examsetrepo.NewAdminRepository(db)
	subjectAdminRepo := subjectrepo.NewSubjectAdminRepository(db)
	questionAdminRepo := questionrepo.NewQuestionAdminRepository(db)
	setQuestionAdminRepo := questionrepo.NewExamSetQuestionAdminRepository(db)

	trackAdminUC := trackadminuc.NewAdminUseCase(trackAdminRepo, trackRepository)
	examSetAdminUC := examsetuc.NewAdminUseCase(examSetAdminRepo, examSetRepository, trackRepository, trackAdminRepo, setQuestionAdminRepo)
	subjectAdminUC := subjectuc.NewSubjectUseCase(subjectAdminRepo)
	questionAdminUC := questionuc.NewAdminUseCase(questionAdminRepo, setQuestionAdminRepo, subjectAdminRepo, examSetRepository, examSetAdminRepo, trackAdminRepo)
	dashboardUC := dashboarduc.NewDashboardUseCase(db)

	examSetQuestionRepo := esqrepo.NewPostgresRepository(db)
	examSetQuestionUC := esquc.NewUseCase(examSetQuestionRepo, questionAdminRepo, examSetRepository, examSetAdminRepo, trackAdminRepo)
	examSetQuestionHandler := esqhttp.NewHandler(examSetQuestionUC)

	importRepository := importrepo.NewRepository(db)
	importUseCase := importuc.NewUseCase(importRepository, subjectAdminRepo, questionAdminRepo)

	importhttp.NewHandler(importUseCase).
		RegisterRoutes(api, authMiddleware, middleware.AdminOnly())
	adminhttp.NewHandler(dashboardUC, trackAdminUC, examSetAdminUC, subjectAdminUC, questionAdminUC, examSetQuestionHandler).
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
