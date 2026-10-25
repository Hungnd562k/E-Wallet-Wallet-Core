package types

import (
	"fmt"
	"testing"
)

func TestValueVsPointer(t *testing.T) {
	w := Wallet{ID: "w1", Balance: 100}

	DepositByValue(w, 50)
	if w.Balance != 100 {
		t.Errorf("value parameter must not change the caller: %v", w.Balance)
	}

	DepositByPointer(&w, 50) // & takes the address
	if w.Balance != 150 {
		t.Errorf("pointer parameter must change the caller: %v", w.Balance)
	}

	w.Freeze() // Go takes &w automatically for pointer-receiver methods
	if w.IsActive() {
		t.Error("frozen wallet reported active")
	}
}

func TestStructCopySharesSlices(t *testing.T) {
	a := Wallet{ID: "a", Tags: []string{"vip"}}
	b := a // copies the struct, but Tags still points at the same array
	b.Tags[0] = "changed"
	if a.Tags[0] != "changed" {
		t.Error("expected the slice backing array to be shared")
	}
	b.ID = "b"
	if a.ID != "a" {
		t.Error("plain fields are copied")
	}
}

func TestZeroValues(t *testing.T) {
	var w Wallet // usable without a constructor
	if w.ID != "" || w.Balance != 0 || w.Status != Active || w.Tags != nil {
		t.Errorf("unexpected zero value %+v", w)
	}
	var m map[string]int // nil map: reads OK, writes panic
	if m["x"] != 0 {
		t.Error("nil map read must return zero")
	}
}

func TestSumGeneric(t *testing.T) {
	if got := Sum([]VND{100, 200}); got != 300 {
		t.Errorf("Sum[VND] = %v", got)
	}
	if got := Sum([]int{1, 2, 3}); got != 6 {
		t.Errorf("Sum[int] = %v", got)
	}
}

func TestGroupByStatus(t *testing.T) {
	g := GroupByStatus([]Wallet{{ID: "a"}, {ID: "b", Status: Frozen}, {ID: "c"}})
	if len(g[Active]) != 2 || len(g[Frozen]) != 1 || len(g[Closed]) != 0 {
		t.Errorf("got %v", g)
	}
}

func TestFilter(t *testing.T) {
	//t.Skip("bài tập: implement Filter then delete this line")
	got := Filter([]Wallet{{ID: "a", Balance: 10}, {ID: "b", Balance: 100}}, 50)
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("got %v", got)
	}
}

func TestFilterGeneric(t *testing.T) {
	// Case struct Wallet
	got := Filter([]Wallet{{ID: "a", Balance: 10}, {ID: "b", Balance: 100}}, 50)
	if len(got) != 1 || got[0].ID != "b" {
		t.Errorf("got %v", got)
	}

	got = Filter(make([]Wallet, 0, 0), 1)
	if len(got) != 0 {
		t.Errorf("Expected result would be empty slice")
	}

	// Case pure number
	gotAny := FilterGeneric([]int16{5, 6, 2, 1, 8, 2}, func(n int16) bool {
		return n%2 == 0
	})
	for _, v := range gotAny {
		if v%2 != 0 {
			t.Errorf("Expected even but got odd number %v", v)
		}
	}
}

func ExampleStatus_String() {
	fmt.Println(Frozen, VND(5000)) // Println uses the String methods
	// Output: FROZEN 5000 VND
}
