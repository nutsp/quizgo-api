package cache

import (
	"fmt"
	"time"
)

const (
	TTLExamTracksList   = 15 * time.Minute
	TTLExamSetsList     = 3 * time.Minute
	TTLExamSetDetail    = 10 * time.Minute
	TTLExamSetsByTrack  = 3 * time.Minute
	TTLHome             = 5 * time.Minute
	TTLUserEntitlements = 3 * time.Minute
	TTLMyExams          = 3 * time.Minute
	TTLUserAccess       = 1 * time.Minute
	TTLResult           = 3 * time.Hour
	TTLIndexBuffer      = 10 * time.Minute
)

func ExamTracksList() string {
	return "exam_tracks:list"
}

func ExamSetsList(hash string) string {
	return "exam_sets:list:" + hash
}

func ExamSetDetail(code string) string {
	return "exam_sets:detail:" + code
}

func ExamSetsByTrack(trackCode, hash string) string {
	return "exam_sets:by_track:" + trackCode + ":" + hash
}

func HomePopularExamSets() string {
	return "home:popular_exam_sets"
}

func HomeFeaturedExamSets() string {
	return "home:featured_exam_sets"
}

func HomeSummary() string {
	return "home:summary"
}

func UserEntitlements(userID string) string {
	return "entitlements:user:" + userID
}

func MyExams(userID string) string {
	return "my_exams:user:" + userID
}

func AccessUserExamSet(userID, examSetID string) string {
	return fmt.Sprintf("access:user:%s:exam_set:%s", userID, examSetID)
}

func ResultSummary(attemptID string) string {
	return "results:attempt:" + attemptID + ":summary"
}

func ResultReview(attemptID string) string {
	return "results:attempt:" + attemptID + ":review"
}

func IndexExamTracks() string {
	return "index:exam_tracks"
}

func IndexExamSetsList() string {
	return "index:exam_sets:list"
}

func IndexExamSet(examSetID string) string {
	return "index:exam_set:" + examSetID
}

func IndexExamSetCode(examSetCode string) string {
	return "index:exam_set_code:" + examSetCode
}

func IndexHome() string {
	return "index:home"
}

func IndexUserAccess(userID string) string {
	return "index:user:" + userID + ":access"
}

func IndexUserMyExams(userID string) string {
	return "index:user:" + userID + ":my_exams"
}

func IndexAttemptResult(attemptID string) string {
	return "index:attempt:" + attemptID + ":result"
}

func LockSubmitAttempt(attemptID string) string {
	return "locks:submit_attempt:" + attemptID
}

func LockDuplicateCreateAttempt(userID, examSetID string) string {
	return fmt.Sprintf("duplicate:create_attempt:%s:%s", userID, examSetID)
}
