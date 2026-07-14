package domain

import "virtual-exam-api/internal/apperrors"

func ValidateAccessConfig(accessType string, priceAmount float64, salePriceAmount *float64, allowSinglePurchase bool) error {
	switch accessType {
	case AccessFree:
		if priceAmount != 0 || allowSinglePurchase {
			return apperrors.ErrInvalidAccessConfig
		}
	case AccessPremium:
		if priceAmount < 0 {
			return apperrors.ErrInvalidAccessConfig
		}
		if allowSinglePurchase && priceAmount <= 0 {
			return apperrors.ErrInvalidAccessConfig
		}
	default:
		return apperrors.ErrInvalidAccessType
	}
	return nil
}

func NormalizeAccessConfig(accessType string, priceAmount float64, allowSinglePurchase bool) (float64, bool) {
	switch accessType {
	case AccessFree:
		return 0, false
	case AccessPremium:
		if !allowSinglePurchase {
			return 0, false
		}
		return priceAmount, true
	default:
		return priceAmount, allowSinglePurchase
	}
}

func IsPublicDiscoveryAccessType(accessType string) bool {
	return accessType != AccessPrivate
}
