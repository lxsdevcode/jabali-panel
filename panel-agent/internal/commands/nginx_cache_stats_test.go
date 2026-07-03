package commands

import (
	"strings"
	"testing"
)

func TestTallyCacheStatuses(t *testing.T) {
	log := "HIT\nHIT\nMISS\nBYPASS\n-\nHIT\nEXPIRED\nREVALIDATED\n\nMISS\n"
	counts, total, ratio := tallyCacheStatuses(strings.NewReader(log))
	// tokens counted (excl "" and "-"): HIT×3, MISS×2, BYPASS×1, EXPIRED×1, REVALIDATED×1 = 8
	if total != 8 {
		t.Fatalf("total=%d want 8 (counts=%v)", total, counts)
	}
	if counts["HIT"] != 3 || counts["MISS"] != 2 || counts["BYPASS"] != 1 || counts["REVALIDATED"] != 1 {
		t.Fatalf("counts wrong: %v", counts)
	}
	// hit = HIT(3)+REVALIDATED(1)=4; cacheable = 4+MISS(2)+EXPIRED(1)=7 → 57.1%
	if ratio != 57.1 {
		t.Fatalf("ratio=%.1f want 57.1", ratio)
	}
	// empty input → 0 ratio, no panic
	if _, tot, r := tallyCacheStatuses(strings.NewReader("")); tot != 0 || r != 0 {
		t.Fatalf("empty: total=%d ratio=%.1f", tot, r)
	}
}
