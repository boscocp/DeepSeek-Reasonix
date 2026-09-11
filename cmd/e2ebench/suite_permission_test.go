package main

import (
	"strings"
	"testing"
)

// The posture reaches the agent's command line exactly once: the CLI rejects a
// run that carries two of them.
func TestBuildRunTaskArgsCarriesPosture(t *testing.T) {
	for _, tc := range []struct{ mode, want string }{
		{mode: "", want: "--permission-mode=workspace-write"},
		{mode: benchmarkPermissionDefault, want: "--permission-mode=workspace-write"},
		{mode: "read-only", want: "--permission-mode=read-only"},
		{mode: "danger-full-access", want: "--permission-mode=danger-full-access"},
	} {
		args := buildRunTaskArgs(suiteConfig{permission: tc.mode}, "/m.json", "", 0, "do it")
		var seen int
		for _, a := range args {
			if strings.HasPrefix(a, "--permission-mode=") {
				seen++
				if a != tc.want {
					t.Fatalf("mode %q produced %q, want %q", tc.mode, a, tc.want)
				}
			}
		}
		if seen != 1 {
			t.Fatalf("mode %q produced %d posture flags, want exactly 1: %v", tc.mode, seen, args)
		}
	}
}

func TestPermissionFlagRejectsAnUnknownPreset(t *testing.T) {
	for _, mode := range []string{"auto", "yolo", "bypassPermissions", "nonsense"} {
		if _, err := permissionFlag(mode); err == nil {
			t.Fatalf("permissionFlag(%q) accepted an unknown preset", mode)
		}
	}
}

// A posture that dropped the approval gate must be visible where the numbers
// are read: two arms otherwise render byte-identical headers.
func TestReportHeaderNamesANonDefaultPosture(t *testing.T) {
	for _, tc := range []struct{ posture, want string }{
		{posture: benchmarkPermissionDefault, want: "## 🤖 Reasonix e2e benchmark (arm `full`)"},
		{posture: "danger-full-access", want: "## 🤖 Reasonix e2e benchmark (arm `full` · danger-full-access-permission)"},
	} {
		got, _, _ := strings.Cut(render([]result{{Permission: tc.posture}}), "\n")
		if got != tc.want {
			t.Fatalf("posture %q rendered %q, want %q", tc.posture, got, tc.want)
		}
	}
}
