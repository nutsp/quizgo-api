package readiness

import (
	"fmt"
	"math"
)

const (
	StatusCollecting  = "collecting"
	StatusReady       = "ready"
	StatusUnavailable = "unavailable"
)

type Config struct {
	Version                  string
	MinimumAnsweredQuestions int
	AccuracyWeight           float64
	CoverageWeight           float64
	RetentionWeight          float64
	TimeWeight               float64
}

func DefaultConfig() Config {
	return Config{
		Version:                  "readiness-v1",
		MinimumAnsweredQuestions: 20,
		AccuracyWeight:           40,
		CoverageWeight:           25,
		RetentionWeight:          20,
		TimeWeight:               15,
	}
}

type SectionEvidence struct {
	WeightPercent  float64
	Answered       int
	Correct        int
	RecentAnswered int
	RecentCorrect  int
}

type Input struct {
	BlueprintReady         bool
	Sections               []SectionEvidence
	TotalAnswered          int
	DurationLimitSeconds   float64
	AverageDurationSeconds float64
}

type Components struct {
	WeightedAccuracy float64 `json:"weighted_accuracy"`
	Coverage         float64 `json:"coverage"`
	RecentRetention  float64 `json:"recent_retention"`
	TimeManagement   float64 `json:"time_management"`
}

type Evidence struct {
	AnsweredQuestions int `json:"answered_questions"`
	RequiredQuestions int `json:"required_questions"`
	CoveredSections   int `json:"covered_sections"`
	TotalSections     int `json:"total_sections"`
}

type Result struct {
	Status             string     `json:"status"`
	Score              *float64   `json:"score,omitempty"`
	CalculationVersion string     `json:"calculation_version"`
	Components         Components `json:"components"`
	Evidence           Evidence   `json:"evidence"`
	Explanations       []string   `json:"explanations"`
}

type Calculator struct{ config Config }

func NewCalculator(config Config) Calculator { return Calculator{config: config} }

func (c Calculator) Calculate(input Input) Result {
	result := Result{
		Status:             StatusUnavailable,
		CalculationVersion: c.config.Version,
		Explanations:       []string{},
		Evidence: Evidence{
			RequiredQuestions: c.config.MinimumAnsweredQuestions,
			TotalSections:     len(input.Sections),
		},
	}
	if !input.BlueprintReady || len(input.Sections) == 0 {
		result.Explanations = append(result.Explanations, "ยังไม่มี Blueprint ที่ผ่านการตรวจทาน")
		return result
	}

	answered := input.TotalAnswered
	coveredWeight := 0.0
	weightedAccuracy := 0.0
	recentAnswered := 0
	recentCorrect := 0
	for _, section := range input.Sections {
		if input.TotalAnswered == 0 {
			answered += section.Answered
		}
		if section.Answered > 0 {
			result.Evidence.CoveredSections++
			coveredWeight += section.WeightPercent
			weightedAccuracy += section.WeightPercent * percent(section.Correct, section.Answered)
		}
		recentAnswered += section.RecentAnswered
		recentCorrect += section.RecentCorrect
	}
	result.Evidence.AnsweredQuestions = answered
	result.Components.Coverage = clamp(coveredWeight)
	if coveredWeight > 0 {
		result.Components.WeightedAccuracy = clamp(weightedAccuracy / coveredWeight)
	}
	result.Components.RecentRetention = clamp(percent(recentCorrect, recentAnswered))
	if input.DurationLimitSeconds > 0 && input.AverageDurationSeconds > 0 {
		result.Components.TimeManagement = clamp(input.DurationLimitSeconds / input.AverageDurationSeconds * 100)
	}

	if answered < c.config.MinimumAnsweredQuestions {
		result.Status = StatusCollecting
		result.Explanations = append(result.Explanations,
			fmt.Sprintf("กำลังเก็บข้อมูลอีก %d ข้อก่อนคำนวณความพร้อม", c.config.MinimumAnsweredQuestions-answered))
		return result
	}

	score := (result.Components.WeightedAccuracy*c.config.AccuracyWeight +
		result.Components.Coverage*c.config.CoverageWeight +
		result.Components.RecentRetention*c.config.RetentionWeight +
		result.Components.TimeManagement*c.config.TimeWeight) / 100
	score = round1(clamp(score))
	result.Score = &score
	result.Status = StatusReady
	result.Explanations = buildExplanations(result.Components)
	return result
}

func buildExplanations(components Components) []string {
	explanations := make([]string, 0, 2)
	if components.Coverage < 80 {
		explanations = append(explanations, "ควรเพิ่มการฝึกในหัวข้อที่ยังไม่ครอบคลุมตาม Blueprint")
	}
	if components.WeightedAccuracy < 70 {
		explanations = append(explanations, "ความแม่นยำตามน้ำหนักหัวข้อยังเป็นส่วนที่ควรพัฒนา")
	}
	if components.RecentRetention < components.WeightedAccuracy-10 {
		explanations = append(explanations, "ความแม่นยำช่วงล่าสุดต่ำกว่าภาพรวม ควรทบทวนหัวข้อที่เคยทำผิด")
	}
	if components.TimeManagement < 80 {
		explanations = append(explanations, "ควรฝึกทำข้อสอบภายใต้เวลาที่กำหนด")
	}
	if len(explanations) == 0 {
		explanations = append(explanations, "หลักฐานครอบคลุมดี รักษาความสม่ำเสมอและทบทวนจุดอ่อนต่อเนื่อง")
	}
	return explanations
}

func percent(part, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func clamp(value float64) float64 {
	return math.Max(0, math.Min(100, value))
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }
