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

// DateToTime converts a Date to midnight UTC of that day.
func DateToTime(d Date) time.Time {
	return time.Date(d.Year, time.Month(d.Month), d.Day, 0, 0, 0, 0, time.UTC)
}

// TimeToPgTypeDate converts t to pgtype.Date. The zero time maps to SQL NULL.
func TimeToPgTypeDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t, Valid: !t.IsZero()}
}
