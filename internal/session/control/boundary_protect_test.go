package control

import (
	"errors"
	"runtime"
	"testing"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/config"
	"reasonix/internal/safety/sandbox"
)

// The settings switch is the config key: off persists, reads back, and survives
// a later save of the other sandbox fields.
func TestSandboxSettingsPersistChangedFileProtection(t *testing.T) {
	t.Setenv("REASONIX_HOME", testenv.TempDir(t))
	c := &Controller{controllerDeps: controllerDeps{workspaceRoot: testenv.TempDir(t)}}

	s := c.SandboxSettings()
	if !s.ProtectChangedFiles {
		t.Fatal("changed-file protection should default on")
	}
	// The subject is the protection switch, not the jail: a host without an OS
	// sandbox refuses every save that asks for one.
	s.Bash, s.ProtectChangedFiles = "off", false
	if err := c.SaveSandboxSettings(s); err != nil {
		t.Fatal(err)
	}
	if c.SandboxSettings().ProtectChangedFiles {
		t.Fatal("turning it off did not read back")
	}
	if config.LoadForEdit(config.UserConfigPath()).Tools.ChangedFilesProtected() {
		t.Fatal("turning it off did not reach the config file")
	}

	s = c.SandboxSettings()
	s.AllowWrite = []string{"/tmp/scratch"}
	if err := c.SaveSandboxSettings(s); err != nil {
		t.Fatal(err)
	}
	if c.SandboxSettings().ProtectChangedFiles {
		t.Fatal("saving another sandbox field turned protection back on")
	}
}

// Where there is no OS sandbox, a settings file that already asks for one (the
// rendered template writes bash = "enforce") still saves its other fields;
// only switching the jail on is refused.
func TestSandboxSettingsSaveWhereNoSandboxExists(t *testing.T) {
	if sandbox.Available() || runtime.GOOS == "windows" {
		t.Skip("needs a macOS/Linux host without an OS sandbox")
	}
	t.Setenv("REASONIX_HOME", testenv.TempDir(t))
	c := &Controller{controllerDeps: controllerDeps{workspaceRoot: testenv.TempDir(t)}}
	s := c.SandboxSettings()
	if err := c.SaveSandboxSettings(s); err != nil {
		t.Fatal(err)
	}
	s = c.SandboxSettings()
	if s.Bash != "enforce" {
		t.Skipf("the saved file reads back bash=%q; the case under test does not arise", s.Bash)
	}
	s.AllowWrite = []string{"/tmp/scratch"}
	if err := c.SaveSandboxSettings(s); err != nil {
		t.Fatalf("an unrelated field could not be saved: %v", err)
	}
	s.Bash = "off"
	if err := c.SaveSandboxSettings(s); err != nil {
		t.Fatal(err)
	}
	s.Bash = "enforce"
	if err := c.SaveSandboxSettings(s); !errors.Is(err, ErrSandboxUnavailable) {
		t.Fatalf("switching the jail on without one: err = %v, want ErrSandboxUnavailable", err)
	}
}
