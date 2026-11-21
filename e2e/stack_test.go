//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type stack struct {
	rosetta, airlock, transponder, logbook, jaeger string
	composeFile                                    string
}

func newStack() stack {
	return stack{
		rosetta:     env("ROSETTA_URL", "http://127.0.0.1:4200"),
		airlock:     env("AIRLOCK_URL", "http://127.0.0.1:4100"),
		transponder: env("TRANSPONDER_URL", "http://127.0.0.1:4000"),
		logbook:     env("LOGBOOK_URL", "http://127.0.0.1:8420"),
		jaeger:      env("JAEGER_URL", "http://127.0.0.1:16686"),
		composeFile: env("COMPOSE_FILE", "../deploy/compose/compose.yaml"),
	}
}

func env(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func randHex(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// hop is the trace context and request ID a test sends, to look for on every hop after.
type hop struct {
	traceID, spanID, requestID string
}

func newHop(t *testing.T) hop {
	return hop{traceID: randHex(t, 16), spanID: randHex(t, 8), requestID: "e2e-" + randHex(t, 8)}
}

func (h hop) headers(token string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + token,
		"traceparent":   "00-" + h.traceID + "-" + h.spanID + "-01",
		"X-Request-ID":  h.requestID,
	}
}

// callTimeout bounds every call the test makes, so a stalled service fails the test instead of
// hanging it.
const callTimeout = time.Minute

func dial(t *testing.T, url string, headers map[string]string) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	c, err := client.NewStreamableHttpClient(url, transport.WithHTTPHeaders(headers), transport.WithHTTPTimeout(callTimeout))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Start(ctx); err != nil {
		t.Fatalf("connecting to %s: %v", url, err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "mothership-e2e", Version: "1"}
	if _, err := c.Initialize(ctx, init); err != nil {
		t.Fatalf("initializing %s: %v", url, err)
	}
	return c
}

func callTool(t *testing.T, c *client.Client, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	req := mcp.CallToolRequest{}
	req.Params.Name, req.Params.Arguments = name, args
	res, err := c.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("calling %s: %v", name, err)
	}
	return res
}

func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func listTools(t *testing.T, c *client.Client) []mcp.Tool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	res, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("listing tools: %v", err)
	}
	return res.Tools
}

// stepTimeout bounds one HTTP request or docker command inside eventually.
const stepTimeout = 15 * time.Second

var httpClient = &http.Client{Timeout: stepTimeout}

func getJSON(url string, headers map[string]string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// eventually runs check until it passes, since usage, traces and logs are written after the call.
func eventually(t *testing.T, what string, check func() error) {
	t.Helper()
	deadline := time.Now().Add(time.Minute)
	for {
		err := check()
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: %v", what, err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func compose(s stack, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stepTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "-f", s.composeFile}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return out, nil
}

// logLines returns the JSON lines a service has logged.
func logLines(s stack, service string) ([]map[string]any, error) {
	out, err := compose(s, "logs", "--no-color", "--no-log-prefix", service)
	if err != nil {
		return nil, err
	}
	var lines []map[string]any
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var m map[string]any
		if json.Unmarshal(sc.Bytes(), &m) == nil {
			lines = append(lines, m)
		}
	}
	return lines, sc.Err()
}

func findLine(lines []map[string]any, want map[string]string) map[string]any {
	for _, l := range lines {
		ok := true
		for k, v := range want {
			if s, _ := l[k].(string); s != v {
				ok = false
				break
			}
		}
		if ok {
			return l
		}
	}
	return nil
}
