package main

import (
	"context"
	"log"

	"virtual-exam-api/internal/config"
	"virtual-exam-api/internal/database"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
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
	)

	if err := seed.Run(context.Background(), db); err != nil {
		log.Fatalf("seed: %v", err)
	}
}
