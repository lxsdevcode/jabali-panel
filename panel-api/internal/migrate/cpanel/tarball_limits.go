package cpanel

import (
	"errors"
	"io"
)

// Total extraction caps for migration tarballs (Gitea #478). The per-entry
// MaxEntrySize cap alone does not stop a tarball of many below-cap entries (or
// a high-ratio compressed archive) from filling /var/lib/jabali-migrations. An
// ExtractBudget enforces a cumulative byte cap AND an entry-count cap during
// streaming, aborting mid-extract rather than after the disk is full.
const (
	// MaxTotalExtract bounds cumulative extracted bytes across all entries.
	MaxTotalExtract = 500 << 30 // 500 GiB
	// MaxEntryCount bounds the number of entries (count / many-small-files bomb).
	MaxEntryCount = 5_000_000
)

var (
	ErrTotalExtractCap = errors.New("migration tarball exceeds total extracted-size cap")
	ErrTooManyEntries  = errors.New("migration tarball exceeds entry-count cap")
)

// ExtractBudget is a per-parse cumulative budget. Not safe for concurrent use.
type ExtractBudget struct {
	bytes   int64
	entries int
}

// NewExtractBudget returns a fresh budget seeded with the package caps.
func NewExtractBudget() *ExtractBudget {
	return &ExtractBudget{bytes: MaxTotalExtract, entries: MaxEntryCount}
}

// Charge copies one tar entry's body from r to w, enforcing both the per-entry
// cap (MaxEntrySize) and the remaining total budget. It returns ErrTotalExtractCap
// when the entry would push cumulative extraction past the total cap, and
// ErrTooManyEntries when the entry-count cap is hit — both terminal for the parse.
func (b *ExtractBudget) Charge(w io.Writer, r io.Reader) (int64, error) {
	if b.entries <= 0 {
		return 0, ErrTooManyEntries
	}
	b.entries--
	if b.bytes <= 0 {
		return 0, ErrTotalExtractCap
	}
	capN := int64(MaxEntrySize)
	if b.bytes < capN {
		capN = b.bytes
	}
	// +1 so a body that would exceed the cap is detected rather than silently
	// truncated.
	n, err := io.Copy(w, io.LimitReader(r, capN+1))
	b.bytes -= n
	if err != nil {
		return n, err
	}
	if n > capN {
		return n, ErrTotalExtractCap
	}
	return n, nil
}
