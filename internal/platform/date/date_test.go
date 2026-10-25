package date

import (
	"testing"
	"time"
)

func TestDateToTimeRoundTrip(t *testing.T) {
	d := Date{Year: 2000, Month: 6, Day: 5}
	got := DateToTime(d)
	if want := time.Date(2000, time.June, 5, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("DateToTime(%v) = %v, want %v", d, got, want)
	}
	if back := TimeToDate(got); back != d {
		t.Fatalf("TimeToDate(DateToTime(%v)) = %v", d, back)
	}
}

func TestTimeToPgTypeDateZeroIsNull(t *testing.T) {
	if TimeToPgTypeDate(time.Time{}).Valid {
		t.Fatal("zero time must map to NULL")
	}
	if !TimeToPgTypeDate(DateToTime(Date{Year: 2000, Month: 1, Day: 1})).Valid {
		t.Fatal("non-zero time must be valid")
	}
}
