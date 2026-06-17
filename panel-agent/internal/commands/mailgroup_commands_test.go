package commands

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

func TestMailGroupApply_BadParams(t *testing.T) {
	_, err := mailGroupApplyHandler(context.Background(), json.RawMessage(`not json`))
	requireAgentErrorCode(t, err, agentwire.CodeInvalidArgument)
}

func TestMailGroupApply_BadEmail(t *testing.T) {
	_, err := mailGroupApplyHandler(context.Background(), json.RawMessage(`{"email":"notanemail"}`))
	requireAgentErrorCode(t, err, agentwire.CodeInvalidArgument)
}

func TestMailGroupMembersSet_BadGroupEmail(t *testing.T) {
	_, err := mailGroupMembersSetHandler(context.Background(), json.RawMessage(`{"group_email":"bad"}`))
	requireAgentErrorCode(t, err, agentwire.CodeInvalidArgument)
}

func TestMailGroupDelete_BadGroupEmail(t *testing.T) {
	_, err := mailGroupDeleteHandler(context.Background(), json.RawMessage(`{"group_email":""}`))
	requireAgentErrorCode(t, err, agentwire.CodeInvalidArgument)
}

func TestPatchVal(t *testing.T) {
	if patchVal(true) != true {
		t.Fatalf("patchVal(true) should be true")
	}
	if patchVal(false) != nil {
		t.Fatalf("patchVal(false) should be nil (unset)")
	}
}

// TestMailGroupApply_CreatesGroup drives the happy path through the fake
// JMAP server: domain resolves, the @type:Group create succeeds, and the
// description (display name) update lands. Asserts the returned group id.
func TestMailGroupApply_CreatesGroup(t *testing.T) {
	srv := newJMAPServer(t, map[string]jmapHandler{
		"x:Domain/query": jmapHandlerReturning(jmapQueryResult{IDs: []string{"dom1"}}),
		"x:Account/query": jmapHandlerReturning(jmapQueryResult{IDs: []string{"grp1"}}),
		"x:Account/set": func(args json.RawMessage) (any, *jmapFakeError) {
			s := string(args)
			switch {
			case strings.Contains(s, `"create"`):
				return jmapSetResult{Created: map[string]json.RawMessage{
					"#g": json.RawMessage(`{"id":"grp1"}`),
				}}, nil
			case strings.Contains(s, `"update"`):
				return jmapSetResult{Updated: map[string]json.RawMessage{
					"grp1": json.RawMessage(`null`),
				}}, nil
			default:
				return jmapSetResult{}, nil
			}
		},
	})
	wireJMAP(t, srv)

	out, err := mailGroupApplyHandler(context.Background(),
		json.RawMessage(`{"email":"marketing@example.com","display_name":"Marketing","description":"Team"}`))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	res, ok := out.(mailGroupApplyResult)
	if !ok {
		t.Fatalf("unexpected result type %T", out)
	}
	if !res.Ok || res.GroupID != "grp1" {
		t.Fatalf("want {ok,group_id=grp1}, got %+v", res)
	}
}
