package repository

// The admin Users list sorts server-side against userListCols.Sort — a
// column absent from that whitelist silently falls back to DefaultSort, so
// the UI would render a sort control that does nothing.
//
// disk_used_kb only became sortable once migration 000257 put it on the row
// and the sweeper started filling it in. This test is the link between the
// three: whitelist entry, real column, and the UI's dataIndex.

import (
	"reflect"
	"strings"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/models"
)

func TestUserListCols_AllowsDiskUsedKBSort(t *testing.T) {
	found := false
	for _, c := range userListCols.Sort {
		if c == "disk_used_kb" {
			found = true
		}
	}
	if !found {
		t.Fatalf("disk_used_kb missing from the sort whitelist %v — the Users list "+
			"column would render a sorter that silently falls back to %q",
			userListCols.Sort, userListCols.DefaultSort)
	}
}

// Every whitelisted sort key has to be a real column, or the ORDER BY is a
// SQL error at runtime rather than a compile-time one.
func TestUserListCols_SortKeysAreRealColumns(t *testing.T) {
	cols := map[string]bool{}
	rt := reflect.TypeOf(models.User{})
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("gorm")
		for _, part := range strings.Split(tag, ";") {
			if name, ok := strings.CutPrefix(strings.TrimSpace(part), "column:"); ok {
				cols[name] = true
			}
		}
		// Fields without an explicit column tag use GORM's snake_case of
		// the field name.
		cols[gormDefaultColumnName(rt.Field(i).Name)] = true
	}
	for _, c := range userListCols.Sort {
		if !cols[c] {
			t.Errorf("sort key %q is not a column on models.User — ORDER BY would fail at runtime", c)
		}
	}
}

// gormDefaultColumnName mirrors GORM's NamingStrategy for the simple cases
// present on this model (CamelCase → snake_case, with the initialisms the
// model actually uses).
func gormDefaultColumnName(field string) string {
	var b strings.Builder
	runes := []rune(field)
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && i > 0 {
			prevLower := runes[i-1] >= 'a' && runes[i-1] <= 'z'
			nextLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if prevLower || nextLower {
				b.WriteByte('_')
			}
		}
		if isUpper {
			b.WriteRune(r - 'A' + 'a')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
