// Package syntax: chapter 01 of docs/go-map. Basic syntax through small money helpers.
//
// Note: the directory is "01_syntax" but the package is "syntax" — a package
// name cannot start with a digit, and it need not match the directory.
package syntax

import (
	"strconv"
	"strings"
)

// FormatVND renders 1234567 as "1.234.567 ₫" (Vietnamese thousands separator).
func FormatVND(amount int64) string {
	sign := ""
	if amount < 0 { // if: no parentheses, braces mandatory
		sign = "-"
		amount = -amount
	}

	digits := strconv.FormatInt(amount, 10) // := declares and infers the type
	var b strings.Builder                   // var declares with the zero value (an empty builder)

	for i, r := range digits { // range over a string yields byte index and rune
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return sign + b.String() + " ₫"
}

// FeeTier shows switch without fallthrough and with conditions instead of values.
func FeeTier(amount int64) string {
	switch {
	case amount <= 0:
		return "invalid"
	case amount < 1_000_000: // underscores in numeric literals are allowed
		return "free"
	case amount < 10_000_000:
		return "standard"
	default:
		return "premium"
	}
}

// SumPositive shows the only loop keyword, for, in its three forms.
func SumPositive(amounts ...int64) (total int64) { // variadic param + named result
	for _, a := range amounts { // "_" discards the index
		if a <= 0 {
			continue
		}
		total += a
	}
	return // naked return of the named result; use sparingly
}

// ParseVND is the inverse of FormatVND: "1.234.567 ₫" → 1234567.
// Exercise (docs/go-map/01-cu-phap.md): implement it, then remove the t.Skip in the test.
func ParseVND(s string) (int64, error) {
	// TODO(bài tập): strings.TrimSuffix, strings.ReplaceAll, strconv.ParseInt
	return 0, nil
}
