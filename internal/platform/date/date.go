package date

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Date global type
type Date struct {
	Year  int
	Month int
	Day   int
}

// TODO: need test.
// Convert time.time to Date global type
func TimeToDate(t time.Time) Date {
	y := t.Year()
	m := t.Month()
	d := t.Day()

	return Date{
		Year:  y,
		Month: int(m),
		Day:   d,
	}
}

// TODO: need test.
// Convert time.time to pgtype.Date
func TimeToPgTypeDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: true}
}
