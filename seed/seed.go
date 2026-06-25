package seed

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	attemptrepo "virtual-exam-api/internal/examattempt/repository"
	examsetrepo "virtual-exam-api/internal/examset/repository"
	questionrepo "virtual-exam-api/internal/question/repository"
	trackrepo "virtual-exam-api/internal/examtrack/repository"
	userrepo "virtual-exam-api/internal/user/repository"
)

func Run(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&trackrepo.ExamTrackModel{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		log.Println("seed: data already exists, skipping")
		return EnsureAdminUser(ctx, db)
	}

	log.Println("seed: inserting demo data")

	subjects := seedSubjects()
	for i := range subjects {
		if err := db.WithContext(ctx).Create(&subjects[i]).Error; err != nil {
			return fmt.Errorf("seed subject: %w", err)
		}
	}
	subjectByCode := map[string]uuid.UUID{}
	for _, s := range subjects {
		subjectByCode[s.Code] = s.ID
	}

	tracks := seedTracks()
	for i := range tracks {
		if err := db.WithContext(ctx).Create(&tracks[i]).Error; err != nil {
			return fmt.Errorf("seed track: %w", err)
		}
	}
	trackByCode := map[string]uuid.UUID{}
	for _, t := range tracks {
		trackByCode[t.Code] = t.ID
	}

	setDefs := []struct {
		TrackCode       string
		Code            string
		Title           string
		Desc            string
		AccessType      string
		PriceAmount     float64
		SalePriceAmount *float64
		CoverImageURL   string
		IsFeatured      bool
		IsOfficial      bool
		TotalQuestions  int
	}{
		{
			TrackCode:      "gpor",
			Code:           "gpor-set-1",
			Title:          "ก.พ. ชุดที่ 1",
			Desc:           "จำลองข้อสอบเสมือนจริงพร้อมจับเวลาและกระดาษคำตอบแบบฝน",
			AccessType:     "free",
			PriceAmount:    0,
			CoverImageURL:  "https://images.unsplash.com/photo-1434030216411-0b793f4b4173",
			IsFeatured:     true,
			IsOfficial:     false,
			TotalQuestions: 100,
		},
		{
			TrackCode:       "gpor",
			Code:            "gpor-set-2",
			Title:           "ก.พ. ชุดที่ 2",
			Desc:            "ชุดข้อสอบ ก.พ. ชุดที่ 2 สำหรับฝึกสอบเสมือนจริง",
			AccessType:      "premium",
			PriceAmount:     199,
			SalePriceAmount: floatPtr(149),
			CoverImageURL:   "https://images.unsplash.com/photo-1456513080510-7bf3a84b82f8",
			IsFeatured:      true,
			IsOfficial:      false,
			TotalQuestions:  100,
		},
		{
			TrackCode:      "police",
			Code:           "police-set-1",
			Title:          "ตร. ชุดที่ 1",
			Desc:           "ชุดข้อสอบตำรวจ ชุดที่ 1 จำลองสนามสอบเสมือนจริง",
			AccessType:     "premium",
			PriceAmount:    249,
			CoverImageURL:  "https://images.unsplash.com/photo-1516321318423-f06f85e504b3",
			IsFeatured:     false,
			IsOfficial:     false,
			TotalQuestions: 100,
		},
		{
			TrackCode:      "police",
			Code:           "police-set-2",
			Title:          "ตร. ชุดที่ 2",
			Desc:           "ชุดข้อสอบตำรวจ ชุดที่ 2",
			AccessType:     "premium",
			PriceAmount:    199,
			CoverImageURL:  "https://images.unsplash.com/photo-1456513080510-7bf3a84b82f8",
			IsFeatured:     false,
			IsOfficial:     false,
			TotalQuestions: 80,
		},
		{
			TrackCode:      "local",
			Code:           "local-set-1",
			Title:          "ท้องถิ่น ชุดที่ 1",
			Desc:           "ชุดข้อสอบท้องถิ่น ชุดที่ 1 สำหรับฝึกสอบเสมือนจริง",
			AccessType:     "free",
			PriceAmount:    0,
			CoverImageURL:  "https://images.unsplash.com/photo-1434030216411-0b793f4b4173",
			IsFeatured:     false,
			IsOfficial:     false,
			TotalQuestions: 80,
		},
		{
			TrackCode:      "gpor",
			Code:           "demo",
			Title:          "ข้อสอบเสมือนจริง ชุด A",
			Desc:           "จำลองข้อสอบเสมือนจริง พร้อมจับเวลาเหมือนสนามจริง",
			AccessType:     "free",
			PriceAmount:    0,
			CoverImageURL:  "https://images.unsplash.com/photo-1516321318423-f06f85e504b3",
			IsFeatured:     true,
			IsOfficial:     true,
			TotalQuestions: 20,
		},
	}

	questionCountPerSet := 20
	allQuestions := buildQuestions(subjectByCode)

	for _, def := range setDefs {
		setID := uuid.New()
		coverURL := def.CoverImageURL
		qCount := def.TotalQuestions
		if qCount > questionCountPerSet {
			qCount = questionCountPerSet
		}

		set := examsetrepo.ExamSetModel{
			ID:              setID,
			ExamTrackID:     trackByCode[def.TrackCode],
			Code:            def.Code,
			Title:           def.Title,
			Description:     def.Desc,
			CoverImageURL:   &coverURL,
			DurationMinutes: 120,
			TotalQuestions:  def.TotalQuestions,
			PassingScore:    60,
			Difficulty:      "medium",
			AccessType:      def.AccessType,
			PriceAmount:     def.PriceAmount,
			Currency:        "THB",
			SalePriceAmount: def.SalePriceAmount,
			Mode:            "mock_exam",
			IsOfficial:      def.IsOfficial,
			IsFeatured:      def.IsFeatured,
			IsActive:        true,
		}
		if err := db.WithContext(ctx).Create(&set).Error; err != nil {
			return fmt.Errorf("seed exam set %s: %w", def.Code, err)
		}

		for qNo := 1; qNo <= qCount; qNo++ {
			qIdx := (qNo - 1) % len(allQuestions)
			qTemplate := allQuestions[qIdx]

			questionID := uuid.New()
			question := questionrepo.QuestionModel{
				ID:           questionID,
				SubjectID:    qTemplate.SubjectID,
				QuestionText: fmt.Sprintf("(%s ข้อ %d) %s", def.Title, qNo, qTemplate.Text),
				Explanation:  qTemplate.Explanation,
				Difficulty:   "medium",
			}
			if err := db.WithContext(ctx).Create(&question).Error; err != nil {
				return err
			}

			for _, ch := range qTemplate.Choices {
				choice := questionrepo.ChoiceModel{
					ID:          uuid.New(),
					QuestionID:  questionID,
					ChoiceKey:   ch.Key,
					ChoiceLabel: ch.Label,
					ChoiceText:  ch.Text,
					IsCorrect:   ch.Correct,
				}
				if err := db.WithContext(ctx).Create(&choice).Error; err != nil {
					return err
				}
			}

			esq := questionrepo.ExamSetQuestionModel{
				ID:         uuid.New(),
				ExamSetID:  setID,
				QuestionID: questionID,
				QuestionNo: qNo,
				Score:      1,
			}
			if err := db.WithContext(ctx).Create(&esq).Error; err != nil {
				return err
			}
		}
	}

	for i := range tracks {
		var setCount int64
		var totalQuestions int64
		db.Model(&examsetrepo.ExamSetModel{}).Where("exam_track_id = ?", tracks[i].ID).Count(&setCount)
		db.Model(&examsetrepo.ExamSetModel{}).Where("exam_track_id = ?", tracks[i].ID).
			Select("COALESCE(SUM(total_questions), 0)").Scan(&totalQuestions)
		db.Model(&trackrepo.ExamTrackModel{}).Where("id = ?", tracks[i].ID).Updates(map[string]any{
			"total_exam_sets": setCount,
			"total_questions": totalQuestions,
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	demoUser := userrepo.UserModel{
		ID:           uuid.New(),
		DisplayName:  "Demo User",
		Email:        "demo@example.com",
		PasswordHash: string(hash),
		Role:         "user",
	}
	if err := db.WithContext(ctx).Create(&demoUser).Error; err != nil {
		return fmt.Errorf("seed demo user: %w", err)
	}

	adminUser := userrepo.UserModel{
		ID:           uuid.New(),
		DisplayName:  "Admin",
		Email:        "admin@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := db.WithContext(ctx).Create(&adminUser).Error; err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	// Mark seeded questions as published for public exam APIs
	if err := db.WithContext(ctx).Model(&questionrepo.QuestionModel{}).
		Where("status = ? OR status IS NULL OR status = ''", "draft").
		Updates(map[string]any{"status": "published", "is_active": true}).Error; err != nil {
		return fmt.Errorf("seed question status: %w", err)
	}

	setByCode := map[string]examsetrepo.ExamSetModel{}
	var allSets []examsetrepo.ExamSetModel
	if err := db.WithContext(ctx).Find(&allSets).Error; err != nil {
		return fmt.Errorf("load exam sets: %w", err)
	}
	for _, s := range allSets {
		setByCode[s.Code] = s
	}

	if err := seedDemoAttempts(ctx, db, demoUser.ID, setByCode); err != nil {
		return fmt.Errorf("seed demo attempts: %w", err)
	}

	log.Println("seed: completed successfully")
	return EnsureAdminUser(ctx, db)
}

func EnsureAdminUser(ctx context.Context, db *gorm.DB) error {
	var existing userrepo.UserModel
	err := db.WithContext(ctx).Where("email = ?", "admin@example.com").First(&existing).Error
	if err == nil {
		if existing.Role != "admin" {
			return db.WithContext(ctx).Model(&existing).Update("role", "admin").Error
		}
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	adminUser := userrepo.UserModel{
		ID:           uuid.New(),
		DisplayName:  "Admin",
		Email:        "admin@example.com",
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := db.WithContext(ctx).Create(&adminUser).Error; err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}
	log.Println("seed: ensured admin user admin@example.com")
	return nil
}

func seedSubjects() []questionrepo.SubjectModel {
	defs := []struct{ Code, Name string }{
		{"thai", "ภาษาไทย"},
		{"math", "คณิตศาสตร์"},
		{"law", "กฎหมายราชการ"},
		{"clerical", "งานสารบรรณ"},
		{"computer", "คอมพิวเตอร์"},
		{"english", "ภาษาอังกฤษ"},
	}
	out := make([]questionrepo.SubjectModel, len(defs))
	for i, d := range defs {
		out[i] = questionrepo.SubjectModel{
			ID:   uuid.New(),
			Code: d.Code,
			Name: d.Name,
		}
	}
	return out
}

func seedTracks() []trackrepo.ExamTrackModel {
	defs := []struct{ Code, Name, Desc string }{
		{"gpor", "สอบ ก.พ.", "ข้อสอบสำหรับการสอบ ก.พ. แบบเสมือนจริง"},
		{"police", "สอบตำรวจ", "ข้อสอบสำหรับการสอบตำรวจแบบเสมือนจริง"},
		{"local", "สอบท้องถิ่น", "ข้อสอบสำหรับการสอบท้องถิ่น"},
		{"teacher", "สอบครูผู้ช่วย", "ข้อสอบสำหรับการสอบครูผู้ช่วย"},
	}
	out := make([]trackrepo.ExamTrackModel, len(defs))
	for i, d := range defs {
		out[i] = trackrepo.ExamTrackModel{
			ID:          uuid.New(),
			Code:        d.Code,
			Name:        d.Name,
			Description: d.Desc,
			IsActive:    true,
		}
	}
	return out
}

type choiceTemplate struct {
	Key, Label, Text string
	Correct          bool
}

type questionTemplate struct {
	SubjectCode string
	Text        string
	Explanation string
	Choices     []choiceTemplate
}

func buildQuestions(subjectByCode map[string]uuid.UUID) []struct {
	SubjectID   uuid.UUID
	Text        string
	Explanation string
	Choices     []choiceTemplate
} {
	templates := []questionTemplate{
		{
			SubjectCode: "law",
			Text:        "ข้อใดคือหลักการสำคัญของการบริหารราชการแผ่นดินตามกฎหมาย?",
			Explanation: "การบริหารราชการแผ่นดินต้องเป็นไปตามหลักนิติธรรม ความเป็นธรรม และความโปร่งใส",
			Choices: []choiceTemplate{
				{"A", "ก", "การบริหารโดยไม่ต้องอาศัยกฎหมาย", false},
				{"B", "ข", "การบริหารตามหลักนิติธรรมและความเป็นธรรม", true},
				{"C", "ค", "การบริหารตามอำนาจส่วนตัว", false},
				{"D", "ง", "การบริหารโดยไม่ต้องรับผิดชอบ", false},
			},
		},
		{
			SubjectCode: "clerical",
			Text:        "การจัดทำหนังสือราชการที่ถูกต้องต้องมีองค์ประกอบใดบ้าง?",
			Explanation: "หนังสือราชการต้องมีส่วนราชการเจ้าของเรื่อง ที่ เรื่อง เรียน และลงนาม",
			Choices: []choiceTemplate{
				{"A", "ก", "มีเฉพาะเนื้อหาเรื่อง", false},
				{"B", "ข", "มีส่วนราชการเจ้าของเรื่อง ที่ เรื่อง เรียน และลงนาม", true},
				{"C", "ค", "มีเฉพาะชื่อผู้ลงนาม", false},
				{"D", "ง", "มีเฉพาะวันที่", false},
			},
		},
		{
			SubjectCode: "thai",
			Text:        "คำใดเขียนตามหลักการสะกดคำที่ถูกต้อง?",
			Explanation: "คำว่า 'ทราบ' สะกดด้วย 'รร' ตามหลักการสะกดคำไทย",
			Choices: []choiceTemplate{
				{"A", "ก", "ทราบ", true},
				{"B", "ข", "ทรัพ", false},
				{"C", "ค", "ทรับ", false},
				{"D", "ง", "ทรา", false},
			},
		},
		{
			SubjectCode: "math",
			Text:        "ถ้า x + 15 = 42 แล้ว x มีค่าเท่าใด?",
			Explanation: "x = 42 - 15 = 27",
			Choices: []choiceTemplate{
				{"A", "ก", "25", false},
				{"B", "ข", "27", true},
				{"C", "ค", "57", false},
				{"D", "ง", "17", false},
			},
		},
		{
			SubjectCode: "computer",
			Text:        "โปรแกรมใดใช้สำหรับสร้างเอกสารราชการทั่วไป?",
			Explanation: "Microsoft Word เป็นโปรแกรมประมวลผลคำที่ใช้กันอย่างแพร่หลาย",
			Choices: []choiceTemplate{
				{"A", "ก", "Microsoft Word", true},
				{"B", "ข", "Adobe Photoshop", false},
				{"C", "ค", "AutoCAD", false},
				{"D", "ง", "WinRAR", false},
			},
		},
		{
			SubjectCode: "english",
			Text:        "Choose the correct sentence.",
			Explanation: "'She works at the ministry' is grammatically correct present simple.",
			Choices: []choiceTemplate{
				{"A", "ก", "She work at the ministry.", false},
				{"B", "ข", "She works at the ministry.", true},
				{"C", "ค", "She working at the ministry.", false},
				{"D", "ง", "She work at ministry.", false},
			},
		},
		{
			SubjectCode: "law",
			Text:        "พระราชบัญญัติข้อมูลข่าวสารของราชการ พ.ศ. 2540 มีวัตถุประสงค์หลักเพื่ออะไร?",
			Explanation: "เปิดเผยข้อมูลข่าวสารของราชการต่อสาธารณะ",
			Choices: []choiceTemplate{
				{"A", "ก", "ปิดบังข้อมูลราชการ", false},
				{"B", "ข", "เปิดเผยข้อมูลข่าวสารของราชการ", true},
				{"C", "ค", "จำกัดสิทธิในการร้องเรียน", false},
				{"D", "ง", "ยกเลิกการตรวจสอบ", false},
			},
		},
		{
			SubjectCode: "clerical",
			Text:        "การลงรหัสหนังสือราชการควรทำเมื่อใด?",
			Explanation: "ลงรหัสเมื่อออกหนังสือราชการอย่างเป็นทางการ",
			Choices: []choiceTemplate{
				{"A", "ก", "เมื่อออกหนังสือราชการอย่างเป็นทางการ", true},
				{"B", "ข", "เมื่อร่างหนังสือเท่านั้น", false},
				{"C", "ค", "เมื่อส่งอีเมลส่วนตัว", false},
				{"D", "ง", "ไม่จำเป็นต้องลงรหัส", false},
			},
		},
		{
			SubjectCode: "thai",
			Text:        "สำนวนไทย 'น้ำขึ้นให้รีบตัก' หมายความว่าอย่างไร?",
			Explanation: "หมายถึงเมื่อมีโอกาสดีควรรีบใช้ประโยชน์",
			Choices: []choiceTemplate{
				{"A", "ก", "เมื่อมีโอกาสดีควรรีบใช้ประโยชน์", true},
				{"B", "ข", "ควรประหยัดน้ำ", false},
				{"C", "ค", "ควรรอให้น้ำลด", false},
				{"D", "ง", "ควรเล่นน้ำ", false},
			},
		},
		{
			SubjectCode: "math",
			Text:        "25% ของ 200 เท่ากับเท่าใด?",
			Explanation: "25% ของ 200 = 0.25 × 200 = 50",
			Choices: []choiceTemplate{
				{"A", "ก", "25", false},
				{"B", "ข", "50", true},
				{"C", "ค", "75", false},
				{"D", "ง", "100", false},
			},
		},
	}

	out := make([]struct {
		SubjectID   uuid.UUID
		Text        string
		Explanation string
		Choices     []choiceTemplate
	}, len(templates))

	for i, t := range templates {
		out[i] = struct {
			SubjectID   uuid.UUID
			Text        string
			Explanation string
			Choices     []choiceTemplate
		}{
			SubjectID:   subjectByCode[t.SubjectCode],
			Text:        t.Text,
			Explanation: t.Explanation,
			Choices:     t.Choices,
		}
	}
	return out
}

func floatPtr(v float64) *float64 {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func seedDemoAttempts(
	ctx context.Context,
	db *gorm.DB,
	userID uuid.UUID,
	setByCode map[string]examsetrepo.ExamSetModel,
) error {
	type attemptDef struct {
		SetCode      string
		ScorePercent float64
		DaysAgo      int
		DurationSec  int
	}

	defs := []attemptDef{
		{"gpor-set-1", 55, 14, 7200},
		{"gpor-set-1", 72, 7, 6600},
		{"gpor-set-1", 80, 1, 6480},
		{"gpor-set-2", 61, 10, 6300},
		{"gpor-set-2", 74, 3, 6120},
		{"police-set-1", 58, 5, 6000},
		{"local-set-1", 70, 2, 5400},
	}

	now := time.Now().UTC()
	for _, def := range defs {
		set, ok := setByCode[def.SetCode]
		if !ok {
			continue
		}

		totalQ := 20
		if set.TotalQuestions < totalQ {
			totalQ = set.TotalQuestions
		}
		correct := int(def.ScorePercent / 100 * float64(totalQ))
		wrong := totalQ - correct
		if wrong < 0 {
			wrong = 0
		}

		started := now.AddDate(0, 0, -def.DaysAgo).Add(-time.Duration(def.DurationSec) * time.Second)
		submitted := started.Add(time.Duration(def.DurationSec) * time.Second)

		attempt := attemptrepo.ExamAttemptModel{
			ID:              uuid.New(),
			UserID:          userID,
			ExamTrackID:     set.ExamTrackID,
			ExamSetID:       set.ID,
			Status:          "submitted",
			StartedAt:       started,
			SubmittedAt:     &submitted,
			ExpiresAt:       started.Add(120 * time.Minute),
			DurationSeconds: intPtr(def.DurationSec),
			Score:           float64(correct),
			TotalScore:      float64(totalQ),
			ScorePercent:    def.ScorePercent,
			CorrectCount:    correct,
			WrongCount:      wrong,
			UnansweredCount: 0,
		}
		if err := db.WithContext(ctx).Create(&attempt).Error; err != nil {
			return err
		}
	}

	return nil
}
