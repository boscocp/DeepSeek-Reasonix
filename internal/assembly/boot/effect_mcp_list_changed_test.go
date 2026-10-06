package boot

// What the model can reach after a server rewrites its catalog is a property of
// the whole assembly: the client that hears the notice, the registry it
// re-registers into, and the request built from it.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"reasonix/internal/contract/event"
	"reasonix/internal/contract/provider"
	"reasonix/internal/session/control"
)

// helperListChangedResult answers the stdio helper's MCP methods. Before the
// first call it offers alpha and beta; after it, beta is gone and gamma and
// delta have arrived.
func helperListChangedResult(method string, mutated bool) any {
	switch method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo":      map[string]any{"name": "live", "version": "1"},
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
		}
	case "tools/list":
		return map[string]any{"tools": listChangedCatalog(mutated)}
	case "tools/call":
		return map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
	}
	return map[string]any{}
}

func listChangedCatalog(mutated bool) []map[string]any {
	names := []string{"alpha", "beta"}
	if mutated {
		names = []string{"alpha", "gamma", "delta"}
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]any{
			"name":        name,
			"description": "tool " + name,
			"inputSchema": map[string]any{"type": "object"},
		})
	}
	return out
}

// listChangedServerBlock is the [[plugins]] entry for one transport. The HTTP
// server announces in the SSE stream of the call it is answering, which is
// where a notification reaches a client that opens no standalone stream.
func listChangedServerBlock(t *testing.T, transport string) string {
	t.Helper()
	if transport == "stdio" {
		return fmt.Sprintf(`
[[plugins]]
name = "live"
type = "stdio"
command = %q
args = ["-test.run=TestHelperProcess", "--"]
load = "always"
[plugins.env]
GO_WANT_HELPER_PROCESS = "1"
GO_WANT_HELPER_LIST_CHANGED = "1"
`, os.Args[0])
	}
	var mutated atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     *int   `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := helperListChangedResult(req.Method, mutated.Load())
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *req.ID, "result": result})
		if req.Method != "tools/call" || !mutated.CompareAndSwap(false, true) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/tools/list_changed\"}\n\n")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", body)
	}))
	t.Cleanup(srv.Close)
	return fmt.Sprintf(`
[[plugins]]
name = "live"
type = "http"
url = %q
load = "always"
`, srv.URL)
}

func listChangedConfig(kind, serverBlock string) string {
	return fmt.Sprintf(`
default_model = "test-model"
[agent]
system_prompt = "BASE"
[codegraph]
enabled = false
[[providers]]
name = "test-model"
kind = %q
model = "x"
`, kind) + serverBlock
}

// listChangedProvider issues one scripted use_capability call per round and
// finishes the turn on any round the script leaves out. Rounds are counted
// across turns, so the script says which turn each call lands in.
type listChangedProvider struct {
	mu    sync.Mutex
	reqs  []provider.Request
	round int
	calls map[int]string
}

func (p *listChangedProvider) Name() string { return "boot-mcp-list-changed" }

func (p *listChangedProvider) Stream(_ context.Context, req provider.Request) (<-chan provider.Chunk, error) {
	p.mu.Lock()
	p.reqs = append(p.reqs, req)
	round := p.round
	p.round++
	call := p.calls[round]
	p.mu.Unlock()

	ch := make(chan provider.Chunk, 2)
	if call != "" {
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: fmt.Sprintf("probe-%d", round), Name: "use_capability", Arguments: call,
		}}
	} else {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "ok"}
	}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	close(ch)
	return ch, nil
}

func (p *listChangedProvider) requests() []provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.reqs)
}

func capabilityCall(id string) string {
	return fmt.Sprintf(`{"action":"call","capability_id":%q,"arguments":{}}`, id)
}

func TestEffectMCPListChangedReachesTheNextRequest(t *testing.T) {
	for _, transport := range []string{"stdio", "http"} {
		t.Run(transport, func(t *testing.T) {
			home := isolateConfigHome(t)
			reasonixHome := filepath.Join(home, ".reasonix")
			t.Setenv("REASONIX_HOME", reasonixHome)
			workspace := robustTempDir(t)
			t.Chdir(workspace)

			kind := "boot-mcp-list-changed-" + transport
			rec := &listChangedProvider{calls: map[int]string{
				0: capabilityCall("mcp-tool:live/alpha"),
				2: `{"action":"inspect","capability_id":"mcp-server:live"}`,
				3: capabilityCall("mcp-tool:live/gamma"),
				4: capabilityCall("mcp-tool:live/beta"),
			}}
			provider.Register(kind, func(provider.Config) (provider.Provider, error) { return rec, nil })
			writeFile(t, workspace, "reasonix.toml", listChangedConfig(kind, listChangedServerBlock(t, transport)))
			approveWorkspace(t, workspace)
			approveProjectServer(t, workspace, "live")

			ctrl, err := Build(t.Context(), Options{Sink: event.Discard, Home: reasonixHome, WorkspaceRoot: workspace})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}
			t.Cleanup(ctrl.Close)

			// The first turn calls alpha, which connects the server and is what
			// makes it announce the catalog it moved to.
			if err := ctrl.Run(t.Context(), "call the live tool"); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := toolResults(rec.requests())["probe-0"]; !strings.Contains(got, "ok") {
				t.Fatalf("the first call returned %q, want the server's result", got)
			}
			waitForRegisteredTool(t, ctrl, "mcp__live__gamma")

			// The second turn reads the catalog, calls what arrived, then what left.
			if err := ctrl.Run(t.Context(), "read the catalog and call both tools"); err != nil {
				t.Fatalf("Run: %v", err)
			}
			requests := rec.requests()
			results := toolResults(requests)
			catalog := results["probe-2"]
			for _, want := range []string{"gamma", "delta"} {
				if !strings.Contains(catalog, want) {
					t.Fatalf("catalog after the notice = %q, missing %q", catalog, want)
				}
			}
			if strings.Contains(catalog, "beta") {
				t.Fatalf("catalog after the notice still offers the dropped tool: %q", catalog)
			}
			if got := results["probe-3"]; !strings.Contains(got, "ok") {
				t.Fatalf("call to the tool the server added returned %q, want its result", got)
			}
			if got := results["probe-4"]; !strings.Contains(got, `not found on server "live"`) {
				t.Fatalf("call to the dropped tool returned %q, want the typed not-found failure", got)
			}

			first, last := requests[0], requests[len(requests)-1]
			if before, after := systemMessage(first.Messages), systemMessage(last.Messages); before != after {
				t.Fatalf("the cache-stable prefix moved with the catalog:\n before: %.200q\n  after: %.200q", before, after)
			}
			if got := mcpToolNames(last); len(got) != 0 {
				t.Fatalf("a server discovered during the session pinned %v into the provider schema", got)
			}
		})
	}
}

// mcpToolNames is the live server's share of one request's tool surface.
func mcpToolNames(req provider.Request) []string {
	var out []string
	for _, name := range toolSchemaNames(req.Tools) {
		if strings.HasPrefix(name, "mcp__live__") {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// waitForRegisteredTool waits for the refresh the server announced to reach
// this session's registry, which is where the next request is built from.
func waitForRegisteredTool(t *testing.T, ctrl *control.Controller, name string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, entry := range ctrl.AllToolContractEntries() {
			if entry.Name == name {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never reached the registry after notifications/tools/list_changed", name)
}
