package usecase

import (
	"context"
	"log"

	"gorm.io/gorm"
	dashboardrepo "virtual-exam-api/internal/admin/dashboard/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
	subjectrepo "virtual-exam-api/internal/subject/repository"
)

type DashboardUseCase struct {
	db       *gorm.DB
	userRepo dashboardrepo.Repository
}

func NewDashboardUseCase(db *gorm.DB) *DashboardUseCase {
	return &DashboardUseCase{
		db:       db,
		userRepo: dashboardrepo.NewPostgresRepository(db),
	}
}

type DashboardLatestExamSet struct {
	ID             string `json:"id"`
	Code           string `json:"code"`
	Title          string `json:"title"`
	ExamTrackName  string `json:"exam_track_name,omitempty"`
	TotalQuestions int    `json:"total_questions"`
	IsActive       bool   `json:"is_active"`
	CreatedAt      string `json:"created_at"`
}

type DashboardLatestQuestion struct {
	ID              string `json:"id"`
	QuestionPreview string `json:"question_preview"`
	SubjectName     string `json:"subject_name,omitempty"`
	Status          string `json:"status"`
	CreatedAt       string `json:"created_at"`
}

type DashboardContentMetrics struct {
	ExamTracks          int64 `json:"exam_tracks"`
	ExamSets            int64 `json:"exam_sets"`
	Subjects            int64 `json:"subjects"`
	QuestionsTotal      int64 `json:"questions_total"`
	QuestionsPublished  int64 `json:"questions_published"`
	QuestionsDraft      int64 `json:"questions_draft"`
	ExamSetsActive      int64 `json:"exam_sets_active"`
}

type DashboardUsageMetrics struct {
	AttemptsTotal int64 `json:"attempts_total"`
}

type DashboardResponse struct {
	Content         DashboardContentMetrics        `json:"content"`
	Usage           DashboardUsageMetrics          `json:"usage"`
	Users           dashboardrepo.AdminUserMetrics `json:"users"`
	Charts          dashboardrepo.AdminCharts      `json:"charts"`
	PremiumExamSets int64                          `json:"premium_exam_sets"`
	FreeExamSets    int64                          `json:"free_exam_sets"`
	LatestExamSets  []DashboardLatestExamSet       `json:"latest_exam_sets"`
	LatestQuestions []DashboardLatestQuestion      `json:"latest_questions"`
}

func (uc *DashboardUseCase) Get(ctx context.Context) (*DashboardResponse, error) {
	totalTracks, err := questionrepo.CountAllTracks(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	totalSets, err := questionrepo.CountAllExamSets(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	totalSubjects, err := subjectrepo.CountAllSubjects(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	qAdmin := questionrepo.NewQuestionAdminRepository(uc.db, nil)
	totalQuestions, err := qAdmin.CountAll(ctx)
	if err != nil {
		return nil, err
	}
	totalAttempts, err := questionrepo.CountAllAttempts(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	published, err := qAdmin.CountByStatus(ctx, "published")
	if err != nil {
		return nil, err
	}
	draft, err := qAdmin.CountByStatus(ctx, "draft")
	if err != nil {
		return nil, err
	}
	activeSets, err := examsetrepo.CountActiveSets(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	premiumSets, err := examsetrepo.CountPremiumSets(ctx, uc.db)
	if err != nil {
		return nil, err
	}
	freeSets, err := examsetrepo.CountFreeSets(ctx, uc.db)
	if err != nil {
		return nil, err
	}

	latestSets, err := examsetrepo.ListLatestSets(ctx, uc.db, 5)
	if err != nil {
		return nil, err
	}
	latestSetResp := make([]DashboardLatestExamSet, len(latestSets))
	for i, s := range latestSets {
		item := DashboardLatestExamSet{
			ID:             s.ID.String(),
			Code:           s.Code,
			Title:          s.Title,
			TotalQuestions: s.TotalQuestions,
			IsActive:       s.IsActive,
			CreatedAt:      s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if s.ExamTrack != nil {
			item.ExamTrackName = s.ExamTrack.Name
		}
		latestSetResp[i] = item
	}

	latestQuestions, err := qAdmin.ListLatest(ctx, 5)
	if err != nil {
		return nil, err
	}
	latestQResp := make([]DashboardLatestQuestion, len(latestQuestions))
	for i, q := range latestQuestions {
		item := DashboardLatestQuestion{
			ID:              q.ID.String(),
			QuestionPreview: questionrepo.TruncatePreview(q.QuestionText, 80),
			Status:          q.Status,
			CreatedAt:       q.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if q.Subject != nil {
			item.SubjectName = q.Subject.Name
		}
		latestQResp[i] = item
	}

	userMetrics, err := uc.userRepo.GetAdminUserMetrics(ctx)
	if err != nil {
		log.Printf("admin dashboard: user metrics query failed: %v", err)
		userMetrics = &dashboardrepo.AdminUserMetrics{}
	}

	charts := uc.userRepo.GetAdminCharts(ctx)

	return &DashboardResponse{
		Content: DashboardContentMetrics{
			ExamTracks:         totalTracks,
			ExamSets:           totalSets,
			Subjects:           totalSubjects,
			QuestionsTotal:     totalQuestions,
			QuestionsPublished: published,
			QuestionsDraft:     draft,
			ExamSetsActive:     activeSets,
		},
		Usage: DashboardUsageMetrics{
			AttemptsTotal: totalAttempts,
		},
		Users:           *userMetrics,
		Charts:          charts,
		PremiumExamSets: premiumSets,
		FreeExamSets:    freeSets,
		LatestExamSets:  latestSetResp,
		LatestQuestions: latestQResp,
	}, nil
}
