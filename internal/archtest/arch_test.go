// Package archtest enforces the module dependency rules of architecture §4.3
// by inspecting the real import graph (`go list -json ./...`).
// Test kiến trúc import ở đây, chưa biết sử dụng kiểu gì
package archtest

import (
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const modulePath = "github.com/Hungnd562k/E-Wallet-Wallet-Core"

// allowed lists, for each business module, the other business modules it may
// import (architecture §4.3 diagram). Every module may import platform.
// Adding an edge here is an architecture decision: update the docs too.
var allowed = map[string][]string{
	"payment":        {"ledger", "limit", "wallet", "authz", "banklink", "merchant"},
	"reconciliation": {"ledger", "banklink", "payment"},
	"wallet":         {"ledger"},
	"kyc":            {"identity"},
	"banklink":       {"identity"},
	"authz":          {"identity"},
	"backoffice":     {"payment", "reconciliation", "dispute", "kyc", "risk"},
	"ledger":         {},
	"limit":          {},
	"identity":       {},
	"merchant":       {},
	"risk":           {},
	"dispute":        {},
	"notification":   {},
	"audit":          {},
}

// compositionRoots may import anything, including module sub-packages.
var compositionRoots = []string{"cmd/", "internal/app", "internal/archtest", "test/", "learn/"}

type pkg struct {
	ImportPath string
	Imports    []string
}

// unit identifies the owner of a package: "payment" for internal/payment/...,
// "platform/db" for internal/platform/db/..., "" for anything else.
// sub is the path below the unit ("" for the unit's root package).
func unit(importPath string) (name, sub string) {
	rel, ok := strings.CutPrefix(importPath, modulePath+"/internal/")
	if !ok {
		return "", ""
	}
	parts := strings.Split(rel, "/")
	if parts[0] == "platform" {
		if len(parts) == 1 {
			return "platform", ""
		}
		return "platform/" + parts[1], strings.Join(parts[2:], "/")
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func isPlatform(u string) bool { return u == "platform" || strings.HasPrefix(u, "platform/") }

func loadPackages(t *testing.T) []pkg {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []pkg
	for {
		var p pkg
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func TestModuleDependencies(t *testing.T) {
	for _, p := range loadPackages(t) {
		rel := strings.TrimPrefix(p.ImportPath, modulePath+"/")
		if slices.ContainsFunc(compositionRoots, func(r string) bool { return strings.HasPrefix(rel, r) }) {
			continue
		}
		from, _ := unit(p.ImportPath)
		if from == "" {
			continue
		}

		for _, imp := range p.Imports {
			to, sub := unit(imp)
			if to == "" || to == from {
				continue
			}

			// Rule 1: only a unit's root package (service.go) is public.
			if sub != "" {
				t.Errorf("%s imports %s: sub-packages of %s are private, use its root package", rel, imp, to)
			}

			switch {
			case isPlatform(from) && !isPlatform(to):
				// Rule 3: platform depends on no module.
				t.Errorf("%s imports module %s: platform must not depend on modules", rel, to)
			case isPlatform(to):
				// Everyone may use platform.
			default:
				// Rules 3 and 4: only the edges of the §4.3 graph (which is acyclic).
				deps, known := allowed[from]
				if !known {
					t.Errorf("module %s is not listed in archtest.allowed", from)
				} else if !slices.Contains(deps, to) {
					t.Errorf("%s imports %s: dependency %s -> %s is not allowed (architecture §4.3)", rel, imp, from, to)
				}
			}
		}
	}
}

// TestAllowedGraphIsAcyclic guards the allowlist itself against cycles.
func TestAllowedGraphIsAcyclic(t *testing.T) {
	const (
		unvisited = iota
		inProgress
		done
	)
	state := map[string]int{}
	var visit func(m string, path []string)
	visit = func(m string, path []string) {
		switch state[m] {
		case inProgress:
			t.Fatalf("cycle in allowed dependencies: %s -> %s", strings.Join(path, " -> "), m)
		case done:
			return
		}
		state[m] = inProgress
		for _, d := range allowed[m] {
			visit(d, append(path, m))
		}
		state[m] = done
	}
	for m := range allowed {
		visit(m, nil)
	}
}
