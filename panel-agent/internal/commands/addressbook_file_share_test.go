package commands

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAddressbookACLForLevel(t *testing.T) {
	if r := addressbookACLForLevel("read"); !r["mayRead"] || r["mayWrite"] || r["mayDelete"] {
		t.Errorf("read = %v", r)
	}
	if r := addressbookACLForLevel("readwrite"); !r["mayRead"] || !r["mayWrite"] || r["mayShare"] || r["mayDelete"] {
		t.Errorf("readwrite = %v", r)
	}
	if r := addressbookACLForLevel("admin"); !r["mayWrite"] || !r["mayShare"] || !r["mayDelete"] {
		t.Errorf("admin = %v", r)
	}
	if r := addressbookACLForLevel("bogus"); len(r) != 1 || !r["mayRead"] {
		t.Errorf("unknown must be read-only, got %v", r)
	}
}

func TestFileACLForLevel(t *testing.T) {
	if r := fileACLForLevel("read"); !r["mayRead"] || r["mayAddChildren"] || r["mayDelete"] {
		t.Errorf("read = %v", r)
	}
	rw := fileACLForLevel("readwrite")
	for _, k := range []string{"mayRead", "mayAddChildren", "mayModifyContent", "mayRename"} {
		if !rw[k] {
			t.Errorf("readwrite missing %s: %v", k, rw)
		}
	}
	if rw["mayDelete"] || rw["mayShare"] {
		t.Errorf("readwrite must not grant delete/share: %v", rw)
	}
	if r := fileACLForLevel("admin"); !r["mayDelete"] || !r["mayShare"] {
		t.Errorf("admin = %v", r)
	}
}

func TestShareSetHandlers_Validation(t *testing.T) {
	for name, h := range map[string]func(context.Context, json.RawMessage) (any, error){
		"addressbook": addressbookShareSetHandler,
		"file":        fileShareSetHandler,
	} {
		if _, err := h(context.Background(), json.RawMessage(``)); err == nil {
			t.Errorf("%s: empty params should error", name)
		}
		if _, err := h(context.Background(), json.RawMessage(`{"owner_email":"bad"}`)); err == nil {
			t.Errorf("%s: invalid email should error", name)
		}
	}
}

func TestShareSetHandlers_Registered(t *testing.T) {
	for _, cmd := range []string{"addressbook.share_set", "file.share_set"} {
		if _, ok := Default.handlers[cmd]; !ok {
			t.Errorf("%s not registered", cmd)
		}
	}
}
