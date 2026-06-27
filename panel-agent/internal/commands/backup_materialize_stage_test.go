package commands

import "testing"

// TestMaterializeStageNameRe pins the restic stage-tag whitelist used as a
// path component during backup materialization (Gitea #464): legitimate flat
// stage names pass; path separators, "..", absolute paths, control chars, and
// empty values are rejected so a poisoned repo tag cannot escape the staging
// tree (caller falls back to the safe snapshot ID on rejection).
func TestMaterializeStageNameRe(t *testing.T) {
	valid := []string{"home", "db", "mail", "dns", "manifest", "deadbeef", "stage_1-x"}
	for _, n := range valid {
		if !materializeStageNameRe.MatchString(n) {
			t.Errorf("valid stage %q should be accepted", n)
		}
	}
	invalid := []string{
		"",                 // empty
		"../../etc",        // traversal
		"a/b",              // separator
		"/abs",             // absolute
		"..",               // parent
		"stage\n",          // control char
		"with space",       // space
		"name.dot",         // dot
	}
	for _, n := range invalid {
		if materializeStageNameRe.MatchString(n) {
			t.Errorf("invalid stage %q must be rejected", n)
		}
	}
}
