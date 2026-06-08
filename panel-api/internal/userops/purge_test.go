package userops

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type recordingAgent struct {
	calls  []recordedCall
	retErr error
}

type recordedCall struct {
	method string
	params any
}

func (r *recordingAgent) Call(_ context.Context, method string, params any) (json.RawMessage, error) {
	r.calls = append(r.calls, recordedCall{method: method, params: params})
	return json.RawMessage(`{}`), r.retErr
}

func TestPurgeDomainMail_CallsAgentWithDomain(t *testing.T) {
	ag := &recordingAgent{}
	if err := PurgeDomainMail(context.Background(), ag, "example.test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ag.calls) != 1 {
		t.Fatalf("want 1 agent call, got %d", len(ag.calls))
	}
	if ag.calls[0].method != "mail.domain.purge_accounts" {
		t.Errorf("method = %q, want mail.domain.purge_accounts", ag.calls[0].method)
	}
	p, ok := ag.calls[0].params.(map[string]any)
	if !ok {
		t.Fatalf("params type = %T, want map[string]any", ag.calls[0].params)
	}
	if p["domain"] != "example.test" {
		t.Errorf("domain param = %v, want example.test", p["domain"])
	}
}

func TestPurgeDomainMail_NilAgentNoop(t *testing.T) {
	// A nil AgentCaller interface must be a no-op, not a panic.
	if err := PurgeDomainMail(context.Background(), nil, "example.test"); err != nil {
		t.Fatalf("nil agent should be no-op, got %v", err)
	}
}

func TestPurgeDomainMail_EmptyDomainNoop(t *testing.T) {
	ag := &recordingAgent{}
	if err := PurgeDomainMail(context.Background(), ag, ""); err != nil {
		t.Fatalf("empty domain should be no-op, got %v", err)
	}
	if len(ag.calls) != 0 {
		t.Errorf("empty domain made %d agent calls, want 0", len(ag.calls))
	}
}

func TestPurgeDomainMail_PropagatesAgentError(t *testing.T) {
	want := errors.New("stalwart down")
	ag := &recordingAgent{retErr: want}
	if err := PurgeDomainMail(context.Background(), ag, "example.test"); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
