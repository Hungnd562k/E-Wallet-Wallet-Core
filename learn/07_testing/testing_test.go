package testingdemo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Table-driven test with named subtests: run one with
//
//	go test ./learn/07_testing -run 'TestNormalizePhone/with_spaces' -v
//
// (spaces in subtest names become underscores).
func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"local format", "0901234567", "+84901234567"},
		{"with spaces", "0901 234 567", "+84901234567"},
		{"international", "+84 901-234-567", "+84901234567"},
		{"too short", "090123", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel() // subtests run concurrently; each has its own tt (Go ≥ 1.22)
			if got := NormalizePhone(tt.in); got != tt.want {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// httptest.NewRecorder lets you call a handler without a network.
func TestBalanceHandler(t *testing.T) {
	h := BalanceHandler(func(id string) (int64, bool) { return 500, id == "w1" })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/balance?wallet_id=w1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]int64
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil || body["balance"] != 500 {
		t.Errorf("body %v err %v", body, err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/balance?wallet_id=nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d", rec.Code)
	}
}

// t.TempDir and t.Cleanup: resources are removed automatically after the test.
func TestTempDirAndCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statement.csv")
	if err := os.WriteFile(path, []byte("id,amount\n1,100\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cleaned := false
	t.Cleanup(func() { cleaned = true }) // runs after this test (LIFO order)
	_ = cleaned
}

// assertPhone is a test helper: t.Helper() makes failures point at the caller's line.
func assertPhone(t *testing.T, in, want string) {
	t.Helper()
	if got := NormalizePhone(in); got != want {
		t.Errorf("NormalizePhone(%q) = %q, want %q", in, got, want)
	}
}

func TestWithHelper(t *testing.T) {
	assertPhone(t, "84901234567", "+84901234567")
}

// Benchmark: go test ./learn/07_testing -bench . -benchmem
func BenchmarkNormalizePhone(b *testing.B) {
	for b.Loop() { // Go 1.24+: b.Loop replaces the old `for i := 0; i < b.N; i++`
		NormalizePhone("+84 901-234-567")
	}
}

// Fuzz test: go test ./learn/07_testing -fuzz FuzzNormalizePhone -fuzztime 10s
// Without -fuzz it runs only the seed corpus, like a normal test.
func FuzzNormalizePhone(f *testing.F) {
	f.Add("0901234567")
	f.Add("+84 901 234 567")
	f.Fuzz(func(t *testing.T, in string) {
		out := NormalizePhone(in)
		if out != "" && (!strings.HasPrefix(out, "+84") || len(out) != 12) {
			t.Errorf("NormalizePhone(%q) = %q: invalid output shape", in, out)
		}
		if out != "" && NormalizePhone(out) != out {
			t.Errorf("not idempotent: %q -> %q", out, NormalizePhone(out))
		}
	})
}

// Exercise (docs/go-map/07-testing.md): write TestNormalizePhoneRejectsLandline
// for "024 3826 1234" (Hanoi landline, 11 digits starting with 0). Does the
// current implementation pass? If not, fix it.
