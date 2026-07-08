package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

// countingAgent counts Call invocations so we can assert the TTL cache
// collapses rapid polls into a single agent fan-out.
type countingAgent struct{ calls int32 }

func (c *countingAgent) Call(_ context.Context, cmd string, _ any) (json.RawMessage, error) {
	atomic.AddInt32(&c.calls, 1)
	switch cmd {
	case "system.info":
		return json.RawMessage(`{"hostname":"mx","load_avg":[0.1,0.2,0.3]}`), nil
	case "system.service_details":
		return json.RawMessage(`{"services":[]}`), nil
	default:
		return json.RawMessage(`{}`), nil
	}
}

func doStatus(m *automationMetrics) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/automation/status", nil)
	m.handle(c)
	return w
}

func TestAutomationMetrics_NoAgentFallsBackToStub(t *testing.T) {
	m := newAutomationMetrics(nil)
	w := doStatus(m)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["healthy"] != true {
		t.Errorf("no-agent stub should be healthy:true, got %v", body["healthy"])
	}
	if _, hasSystem := body["system"]; hasSystem {
		t.Error("no-agent stub must not carry system metrics")
	}
}

func TestAutomationMetrics_AggregatesAndCaches(t *testing.T) {
	ag := &countingAgent{}
	m := newAutomationMetrics(ag)

	w := doStatus(m)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["system"]; !ok {
		t.Error("expected system metrics in the payload")
	}
	if body["healthy"] != true {
		t.Errorf("all-collectors-ok should be healthy, got %v", body["healthy"])
	}
	firstCalls := atomic.LoadInt32(&ag.calls)
	if firstCalls == 0 {
		t.Fatal("expected agent calls on the first request")
	}

	// A second immediate poll must be served from the TTL cache — no new
	// agent fan-out.
	_ = doStatus(m)
	if atomic.LoadInt32(&ag.calls) != firstCalls {
		t.Errorf("second poll should hit the cache; calls went %d -> %d", firstCalls, ag.calls)
	}
}
