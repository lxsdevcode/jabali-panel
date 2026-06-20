package commands

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSharedResourceHandlers_Validation(t *testing.T) {
	if _, err := sharedResourceApplyHandler(context.Background(), json.RawMessage(`{"email":"bad"}`)); err == nil {
		t.Error("apply: invalid email should error")
	}
	if _, err := sharedResourceDestroyHandler(context.Background(), json.RawMessage(`{"email":"bad"}`)); err == nil {
		t.Error("destroy: invalid email should error")
	}
	if _, err := sharedResourceApplyHandler(context.Background(), json.RawMessage(`{bad json`)); err == nil {
		t.Error("apply: bad json should error")
	}
}

func TestSharedResourceHandlers_Registered(t *testing.T) {
	for _, cmd := range []string{"sharedresource.apply", "sharedresource.destroy"} {
		if _, ok := Default.handlers[cmd]; !ok {
			t.Errorf("%s not registered", cmd)
		}
	}
}
