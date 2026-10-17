package deps

import (
	"strings"
	"testing"
)

func TestNewPaymentID(t *testing.T) {
	if id := NewPaymentID(); !strings.HasPrefix(id, "txn_") || len(id) != 40 {
		t.Errorf("unexpected id %q", id)
	}
}

func TestBuildInfo(t *testing.T) {
	if v := GoVersion(); !strings.HasPrefix(v, "go1.") {
		t.Errorf("GoVersion() = %q", v)
	}
	// Empty under `go test` (see Dependencies); printed for curiosity with -v.
	t.Logf("deps recorded in this test binary: %d", len(Dependencies()))
}
