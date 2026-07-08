package modules

// Resolve applies dependency + conflict rules to a selection set and returns a
// normalized set: every Requires edge is pulled in, and Conflicts are dropped
// in favor of the just-toggled key. Deterministic, pure — the TUI calls it
// after every toggle so the on-screen ticks always satisfy the rules.
//
// selected is the set of enabled optional-module keys. justEnabled is the key
// the user most recently turned ON ("" when none / on a bulk apply), used to
// break conflicts in favor of the fresh choice.
func Resolve(selected map[string]bool, justEnabled string) map[string]bool {
	out := map[string]bool{}
	for k, v := range selected {
		if v {
			out[k] = true
		}
	}
	// Pull in requirements transitively.
	changed := true
	for changed {
		changed = false
		for key := range out {
			m, ok := byKey(key)
			if !ok {
				continue
			}
			for _, req := range m.Requires {
				if !out[req] {
					out[req] = true
					changed = true
				}
			}
		}
	}
	// Drop conflicts in favor of justEnabled.
	if justEnabled != "" && out[justEnabled] {
		if m, ok := byKey(justEnabled); ok {
			for _, c := range m.Conflicts {
				delete(out, c)
			}
		}
	}
	return out
}

// DependentsOf returns the enabled modules that require `key` — used to warn
// (and cascade) when the user unticks a dependency.
func DependentsOf(key string, selected map[string]bool) []string {
	var deps []string
	for k := range selected {
		if !selected[k] {
			continue
		}
		m, ok := byKey(k)
		if !ok {
			continue
		}
		for _, req := range m.Requires {
			if req == key {
				deps = append(deps, k)
			}
		}
	}
	return deps
}
