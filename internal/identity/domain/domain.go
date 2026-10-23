package domain

import (
	"strings"
	"unicode"
)

type ONBOARDING string

const (
	OnboardingRegistered ONBOARDING = "REGISTERED"
	OnboardingPending    ONBOARDING = "KYC_PENDING"
	OnboardingVerified   ONBOARDING = "KYC_VERIFIED"
)

type KYC_STATUS string

const (
	Registered   KYC_STATUS = "REGISTERED"
	Pending      KYC_STATUS = "KYC_PENDING"
	Verified     KYC_STATUS = "KYC_VERIFIED"
	Rejected     KYC_STATUS = "KYC_REJECTED"
	ManualReview KYC_STATUS = "MANUAL_REVIEW"
)

// NormalizePhone turns "0901 234 567" or "+84 901-234-567" into "+84901234567".
// It returns "" when the input is not a Vietnamese mobile number.
func NormalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "84") && len(d) == 11:
		return "+" + d
	case strings.HasPrefix(d, "0") && len(d) == 10:
		return "+84" + d[1:]
	}
	return ""
}
