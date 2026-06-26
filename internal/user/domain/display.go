package domain

import "strings"

func MaskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		if len(local) == 0 {
			return "***@" + domain
		}
		return local[:1] + "***@" + domain
	}
	return local[:2] + "***@" + domain
}

func PublicDisplayName(displayName, email string) string {
	if strings.TrimSpace(displayName) != "" {
		return strings.TrimSpace(displayName)
	}
	return MaskEmail(email)
}
