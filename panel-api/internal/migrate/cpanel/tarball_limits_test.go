package cpanel

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// TestExtractBudget covers the migration tarball total/entry caps (Gitea #478).
func TestExtractBudget(t *testing.T) {
	// Within the total byte budget: copy succeeds.
	b := &ExtractBudget{bytes: 10, entries: 5}
	n, err := b.Charge(io.Discard, strings.NewReader("12345"))
	if err != nil || n != 5 {
		t.Fatalf("within budget: n=%d err=%v", n, err)
	}
	// Next entry would push past the remaining 5-byte budget → abort.
	_, err = b.Charge(io.Discard, strings.NewReader("123456"))
	if !errors.Is(err, ErrTotalExtractCap) {
		t.Fatalf("expected ErrTotalExtractCap, got %v", err)
	}

	// Entry-count cap: the (count+1)-th entry is refused.
	bc := &ExtractBudget{bytes: 1 << 30, entries: 1}
	if _, err := bc.Charge(io.Discard, strings.NewReader("a")); err != nil {
		t.Fatalf("first entry should pass: %v", err)
	}
	if _, err := bc.Charge(io.Discard, strings.NewReader("b")); !errors.Is(err, ErrTooManyEntries) {
		t.Fatalf("expected ErrTooManyEntries, got %v", err)
	}

	// A zero-budget charge is immediately refused.
	bz := &ExtractBudget{bytes: 0, entries: 5}
	if _, err := bz.Charge(io.Discard, strings.NewReader("x")); !errors.Is(err, ErrTotalExtractCap) {
		t.Fatalf("zero budget must refuse, got %v", err)
	}
}
