package control

import (
	"testing"

	"reasonix/internal/contract/tool"
	"reasonix/internal/ext/plugin"
)

func refreshedTool(name, raw string) catalogTool {
	return catalogTool{name: name, server: "atlas", raw: raw}
}

// A refresh replaces what the server offers rather than adding to it: a tool it
// dropped has to leave the registry, or the model keeps being offered a call
// that can only fail.
func TestRefreshedMCPToolsReplaceTheServerPrefix(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(refreshedTool("mcp__atlas__gone", "gone"))
	reg.Add(refreshedTool("mcp__atlas__kept", "kept"))
	reg.Add(catalogTool{name: "mcp__ledger__post", server: "ledger", raw: "post"})

	registerRefreshedMCPTools(reg, plugin.Spec{Name: "atlas"}, []tool.Tool{
		refreshedTool("mcp__atlas__kept", "kept"),
		refreshedTool("mcp__atlas__added", "added"),
	})

	for name, want := range map[string]bool{
		"mcp__atlas__gone": false, "mcp__atlas__kept": true,
		"mcp__atlas__added": true, "mcp__ledger__post": true,
	} {
		if _, ok := reg.Get(name); ok != want {
			t.Errorf("%s present = %v, want %v", name, ok, want)
		}
	}
}

// A session that suspended a server turned it off. A server announcing new
// tools is not the user asking for it back, so the refresh must not be what
// puts its prefix on screen again.
func TestRefreshedMCPToolsLeaveASuspendedServerOff(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(refreshedTool("mcp__atlas__old", "old"))
	reg.SuspendPrefix(plugin.ToolPrefix("atlas"))

	registerRefreshedMCPTools(reg, plugin.Spec{Name: "atlas"}, []tool.Tool{
		refreshedTool("mcp__atlas__added", "added"),
	})

	if _, ok := reg.Get("mcp__atlas__added"); ok {
		t.Fatal("a refresh brought back a server this session had turned off")
	}
}
