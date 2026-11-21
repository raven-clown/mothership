//go:build e2e

package e2e

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Gaps found by this test, each skipped with its issue until the project closes it.
const (
	gapEchoToolCalls    = "https://github.com/raven-clown/transponder/issues/205"
	gapAirlockHeaderVar = "https://github.com/raven-clown/airlock/issues/59"
	gapLogbookModels    = "https://github.com/raven-clown/logbook/issues/43"
	gapAirlockOIDC      = "https://github.com/raven-clown/airlock/issues/10"
	gapRosettaIdentity  = "https://github.com/raven-clown/rosetta/issues/45"
)

const (
	rosettaKey      = "ros_demo_alice"
	airlockKey      = "alk_demo_alice"
	transponderView = "tp_demo_viewer"
)

// TestAgentAsksTransponder walks the left side of the diagram: a client asks a rosetta
// agent, and the agent gets its reply from transponder.
func TestAgentAsksTransponder(t *testing.T) {
	s, h := newStack(), newHop(t)
	c := dial(t, s.rosetta+"/mcp", h.headers(rosettaKey))
	msg := "remember that run " + h.requestID + " passed"
	res := callTool(t, c, "assistant", map[string]any{"message": msg})
	if res.IsError || !strings.Contains(resultText(res), "echo: "+msg) {
		t.Fatalf("the agent's reply is not transponder's echo of the message: %q", resultText(res))
	}

	t.Run("transponder recorded usage with the request ID", func(t *testing.T) {
		eventually(t, "a usage record with X-Request-ID "+h.requestID, func() error {
			var page struct {
				Items []struct {
					RequestID    string `json:"request_id"`
					Key          string `json:"key"`
					InputTokens  int64  `json:"input_tokens"`
					OutputTokens int64  `json:"output_tokens"`
				} `json:"items"`
			}
			if err := getJSON(s.transponder+"/admin/v1/requests?limit=100", map[string]string{"Authorization": "Bearer " + transponderView}, &page); err != nil {
				return err
			}
			for _, r := range page.Items {
				if r.RequestID != h.requestID {
					continue
				}
				if r.InputTokens == 0 || r.OutputTokens == 0 {
					return fmt.Errorf("the record has no tokens: %+v", r)
				}
				return nil
			}
			return errors.New("no record has it")
		})
	})

	t.Run("traceparent reached transponder", func(t *testing.T) {
		eventually(t, "a transponder span in trace "+h.traceID, func() error {
			return traceHas(s, h.traceID, "transponder")
		})
	})

	t.Run("the agent called a logbook tool through airlock", func(t *testing.T) {
		t.Skip("transponder's echo provider never answers with a tool call (" + gapEchoToolCalls +
			"), and airlock can't reach logbook yet (" + gapAirlockHeaderVar + ")")
	})

	t.Run("rosetta forwarded traceparent and X-Request-ID to airlock", func(t *testing.T) {
		t.Skip("airlock records them only on a tool call, which the agent can't make yet (" + gapEchoToolCalls +
			", " + gapAirlockHeaderVar + ")")
	})
}

// TestToolThroughAirlock walks the right side: a tool call through airlock reaches logbook.
func TestToolThroughAirlock(t *testing.T) {
	s, h := newStack(), newHop(t)
	c := dial(t, s.airlock+"/mcp", h.headers(airlockKey))
	if !hasTool(listTools(t, c), "logbook__remember") {
		if logbookRefusesAirlock(t, s) {
			t.Skip("airlock lists no logbook tools, since it sends ${LOGBOOK_KEY} to logbook unresolved: " + gapAirlockHeaderVar)
		}
		t.Fatal("airlock lists no logbook__remember tool, and its log doesn't show the known cause")
	}
	content := "e2e run " + h.requestID + ": a tool call reached logbook through airlock"
	subject := "e2e." + h.requestID
	if res := callTool(t, c, "logbook__remember", map[string]any{"content": content, "subject": subject}); res.IsError {
		t.Fatalf("logbook__remember failed: %s", resultText(res))
	}

	t.Run("airlock's audit log has the call", func(t *testing.T) {
		eventually(t, "an audit record with X-Request-ID "+h.requestID, func() error {
			lines, err := logLines(s, "airlock")
			if err != nil {
				return err
			}
			l := findLine(lines, map[string]string{"log": "audit", "kind": "tool", "request_id": h.requestID})
			if l == nil {
				return errors.New("no audit record has it")
			}
			for k, want := range map[string]string{"server": "logbook", "tool": "remember", "decision": "allow", "outcome": "ok", "subject": "key:alice", "trace_id": h.traceID} {
				if got, _ := l[k].(string); got != want {
					return fmt.Errorf("the audit record has %s %q, want %q: %v", k, got, want, l)
				}
			}
			return nil
		})
	})

	t.Run("logbook stored the memory", func(t *testing.T) {
		out, err := compose(s, "exec", "-T", "airlock", "cat", "/keys/logbook-airlock.key")
		if err != nil {
			t.Fatal(err)
		}
		key := strings.TrimSpace(string(out))
		var page struct {
			Items []struct {
				Content string `json:"content"`
				Scope   string `json:"scope"`
			} `json:"items"`
		}
		q := url.Values{"scope": {"project:mothership"}, "subject": {subject}}
		if err := getJSON(s.logbook+"/v1/memories?"+q.Encode(), map[string]string{"Authorization": "Bearer " + key}, &page); err != nil {
			t.Fatal(err)
		}
		for _, m := range page.Items {
			if m.Content == content {
				return
			}
		}
		t.Fatalf("no memory in project:mothership with subject %s says %q: %+v", subject, content, page.Items)
	})

	t.Run("airlock forwarded traceparent and X-Request-ID to logbook", func(t *testing.T) {
		eventually(t, "a logbook request log with X-Request-ID "+h.requestID, func() error {
			lines, err := logLines(s, "logbook")
			if err != nil {
				return err
			}
			l := findLine(lines, map[string]string{"msg": "request", "request_id": h.requestID})
			if l == nil {
				return errors.New("no logbook request log has it")
			}
			if got, _ := l["trace_id"].(string); got != h.traceID {
				return fmt.Errorf("logbook logged trace %q, want %q", got, h.traceID)
			}
			return nil
		})
	})

	t.Run("logbook asked transponder for embeddings", func(t *testing.T) {
		t.Skip("logbook has no setting for an embeddings endpoint yet: " + gapLogbookModels)
	})
}

// TestOneIdentity checks that the caller's own token travels the whole path, as the contract
// asks, so every project sees the same user.
func TestOneIdentity(t *testing.T) {
	t.Skip("each project takes only its own API keys, so one caller can't be the same user in all four. " +
		"That needs OIDC in airlock (" + gapAirlockOIDC + ") and rosetta forwarding the caller's token (" + gapRosettaIdentity + ")")
}

// logbookRefusesAirlock reports whether airlock's latest word on the logbook backend is that
// logbook asked for authorization, which is how raven-clown/airlock#59 shows.
func logbookRefusesAirlock(t *testing.T, s stack) bool {
	t.Helper()
	lines, err := logLines(s, "airlock")
	if err != nil {
		t.Fatal(err)
	}
	var last map[string]any
	for _, l := range lines {
		if b, _ := l["backend"].(string); b != "logbook" {
			continue
		}
		if msg, _ := l["msg"].(string); msg == "backend down" || msg == "backend up" {
			last = l
		}
	}
	if last == nil || last["msg"] != "backend down" {
		return false
	}
	e, _ := last["error"].(string)
	return strings.Contains(e, "authorization required")
}

func hasTool(tools []mcp.Tool, name string) bool {
	for _, t := range tools {
		if t.Name == name {
			return true
		}
	}
	return false
}

func traceHas(s stack, traceID, service string) error {
	var res struct {
		Data []struct {
			Processes map[string]struct {
				ServiceName string `json:"serviceName"`
			} `json:"processes"`
		} `json:"data"`
	}
	if err := getJSON(s.jaeger+"/api/traces/"+traceID, nil, &res); err != nil {
		return err
	}
	for _, tr := range res.Data {
		for _, p := range tr.Processes {
			if p.ServiceName == service {
				return nil
			}
		}
	}
	return fmt.Errorf("no %s span in the trace", service)
}
