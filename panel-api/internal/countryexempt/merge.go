package countryexempt

import (
	"net/netip"
	"sort"
)

// mergeCIDRs collapses a CIDR list into a minimal equivalent set: entries
// contained in a broader entry are dropped and adjacent siblings are
// repeatedly coalesced into their parent. mmdb-derived zones arrive
// city-granular (thousands of small blocks for a large country); the merge
// is what keeps the LAPI allowlist compact. Garbage lines are skipped —
// inputs are validated upstream, this is the defensive layer.
func mergeCIDRs(cidrs []string) []string {
	v4 := map[netip.Prefix]struct{}{}
	v6 := map[netip.Prefix]struct{}{}
	for _, c := range cidrs {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			continue
		}
		p = p.Masked()
		if p.Addr().Is4() {
			v4[p] = struct{}{}
		} else {
			v6[p] = struct{}{}
		}
	}
	return append(collapse(v4), collapse(v6)...)
}

// collapse runs the drop-contained + merge-siblings fixpoint over one
// address family. Every pass strictly shrinks the set, so it terminates.
func collapse(set map[netip.Prefix]struct{}) []string {
	for {
		changed := false
		for p := range set {
			if containedByAny(set, p) {
				delete(set, p) // already covered by a broader prefix
				changed = true
				continue
			}
			if p.Bits() == 0 {
				continue
			}
			parent := netip.PrefixFrom(p.Addr(), p.Bits()-1).Masked()
			sib := sibling(p)
			if _, ok := set[sib]; ok {
				delete(set, p)
				delete(set, sib)
				set[parent] = struct{}{}
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p.String())
	}
	sort.Strings(out) // deterministic snapshots and tests
	return out
}

// containedByAny reports whether any strict ancestor of p is in the set.
func containedByAny(set map[netip.Prefix]struct{}, p netip.Prefix) bool {
	for b := p.Bits() - 1; b >= 0; b-- {
		if _, ok := set[netip.PrefixFrom(p.Addr(), b).Masked()]; ok {
			return true
		}
	}
	return false
}

// sibling flips the last significant bit of p: the other half of p's parent.
func sibling(p netip.Prefix) netip.Prefix {
	bits := p.Bits()
	flip := func(b []byte) {
		i := (bits - 1) / 8
		b[i] ^= 1 << (7 - (bits-1)%8)
	}
	if p.Addr().Is4() {
		a := p.Addr().As4()
		flip(a[:])
		return netip.PrefixFrom(netip.AddrFrom4(a), bits).Masked()
	}
	a := p.Addr().As16()
	flip(a[:])
	return netip.PrefixFrom(netip.AddrFrom16(a), bits).Masked()
}
