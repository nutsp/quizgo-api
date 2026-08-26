package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

//go:embed packages.json
var defaultCatalog []byte

type Package struct {
	ID              string       `json:"id"`
	Group           string       `json:"group"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	DurationMonths  int          `json:"durationMonths,omitempty"`
	OriginalPrice   int          `json:"originalPrice,omitempty"`
	SalePrice       int          `json:"salePrice,omitempty"`
	DiscountPercent int          `json:"discountPercent,omitempty"`
	MonthlyPrice    int          `json:"monthlyPrice,omitempty"`
	Currency        string       `json:"currency"`
	Badge           string       `json:"badge,omitempty"`
	Features        []string     `json:"features"`
	Purchasable     bool         `json:"purchasable"`
	ComingSoon      bool         `json:"comingSoon,omitempty"`
	AccessPolicy    AccessPolicy `json:"accessPolicy"`
}

type AccessPolicy struct {
	DailyCompletedAttempts *int   `json:"dailyCompletedAttempts,omitempty"`
	ResultAccess           string `json:"resultAccess,omitempty"`
	ExamCatalog            string `json:"examCatalog,omitempty"`
	Explanations           bool   `json:"explanations"`
	WeaknessAnalysis       bool   `json:"weaknessAnalysis"`
	History                bool   `json:"history"`
}

func LoadDefault() ([]Package, error) {
	var packages []Package
	if err := json.Unmarshal(defaultCatalog, &packages); err != nil {
		return nil, fmt.Errorf("decode package catalog: %w", err)
	}
	for i := range packages {
		p := &packages[i]
		if p.ID == "" || p.Group == "" || p.Currency == "" {
			return nil, fmt.Errorf("package at index %d is incomplete", i)
		}
		if p.Purchasable {
			if p.DurationMonths <= 0 || p.OriginalPrice <= 0 || p.SalePrice <= 0 || p.SalePrice > p.OriginalPrice {
				return nil, fmt.Errorf("package %s has invalid price or duration", p.ID)
			}
			p.DiscountPercent = int(math.Round(float64(p.OriginalPrice-p.SalePrice) / float64(p.OriginalPrice) * 100))
			p.MonthlyPrice = int(math.Round(float64(p.SalePrice) / float64(p.DurationMonths)))
		}
	}
	return packages, nil
}

func Find(packages []Package, id string) (Package, bool) {
	for _, item := range packages {
		if item.ID == id {
			return item, true
		}
	}
	return Package{}, false
}
