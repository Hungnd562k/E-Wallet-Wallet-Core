package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestPostingValidate(t *testing.T) {
	a, b := uuid.New(), uuid.New()

	tests := []struct {
		name    string
		lines   []Line
		wantErr error // nil = valid; ErrUnbalanced = must wrap it; otherwise any error
	}{
		{
			name:  "balanced transfer",
			lines: []Line{{a, Debit, 100}, {b, Credit, 100}},
		},
		{
			name:  "split credit",
			lines: []Line{{a, Debit, 100}, {b, Credit, 60}, {a, Credit, 40}},
		},
		{
			name:    "unbalanced",
			lines:   []Line{{a, Debit, 100}, {b, Credit, 90}},
			wantErr: ErrUnbalanced,
		},
		{
			name:    "single line",
			lines:   []Line{{a, Debit, 100}},
			wantErr: ErrUnbalanced,
		},
		{
			name:    "zero amount",
			lines:   []Line{{a, Debit, 0}, {b, Credit, 0}},
			wantErr: errAny,
		},
		{
			name:    "bad direction",
			lines:   []Line{{a, "X", 100}, {b, Credit, 100}},
			wantErr: errAny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Posting{Kind: KindTransfer, Currency: "VND", Lines: tt.lines}.Validate()
			switch {
			case tt.wantErr == nil && err != nil:
				t.Fatalf("want valid, got %v", err)
			case tt.wantErr == errAny && err == nil:
				t.Fatal("want error, got nil")
			case tt.wantErr != nil && tt.wantErr != errAny && !errors.Is(err, tt.wantErr):
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
		})
	}
}

var errAny = errors.New("any error")

func TestApply(t *testing.T) {
	tests := []struct {
		name   string
		normal Direction
		line   Line
		want   int64
	}{
		{"credit on liability increases", Credit, Line{Direction: Credit, Amount: 30}, 130},
		{"debit on liability decreases", Credit, Line{Direction: Debit, Amount: 30}, 70},
		{"debit on asset increases", Debit, Line{Direction: Debit, Amount: 30}, 130},
		{"credit on asset decreases", Debit, Line{Direction: Credit, Amount: 30}, 70},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Apply(tt.normal, 100, tt.line); got != tt.want {
				t.Errorf("Apply = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestAccountIDsDeduplicates(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	p := Posting{Lines: []Line{{a, Debit, 1}, {b, Credit, 1}, {a, Credit, 1}}}
	if got := p.AccountIDs(); len(got) != 2 {
		t.Fatalf("want 2 distinct ids, got %v", got)
	}
}
