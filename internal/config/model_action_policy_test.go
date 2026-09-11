package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadProviderEntryForActionPolicy(t *testing.T, providerBody string) *ProviderEntry {
	t.Helper()
	dir := t.TempDir()
	body := "default_model = \"test-model\"\n\n[[providers]]\nname = \"test-model\"\nkind = \"openai\"\nmodel = \"x\"\n" + providerBody + "\n"
	if err := os.WriteFile(filepath.Join(dir, "reasonix.toml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	c, err := LoadForRootReadOnly(dir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	e, ok := c.ResolveModel("test-model")
	if !ok {
		t.Fatal("ResolveModel did not resolve the configured provider")
	}
	return e
}

func TestActionPolicyPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "unset", want: false},
		{
			name: "model override on",
			body: `model_overrides = { "x" = { action_policy = true } }`,
			want: true,
		},
		{
			name: "model override off",
			body: `model_overrides = { "x" = { action_policy = false } }`,
			want: false,
		},
		{
			name: "override for another model does not apply",
			body: `model_overrides = { "other" = { action_policy = true } }`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := loadProviderEntryForActionPolicy(t, tc.body)
			if got := AppliesModelActionPolicy(e); got != tc.want {
				t.Fatalf("AppliesModelActionPolicy = %v, want %v", got, tc.want)
			}
		})
	}
}

// normalizedModelOverrides drops overrides it considers empty. An override that
// only carries action_policy must survive that pass, or the key silently does
// nothing when it is the sole entry.
func TestActionPolicyOnlyOverrideSurvivesLoad(t *testing.T) {
	e := loadProviderEntryForActionPolicy(t, `model_overrides = { "x" = { action_policy = true } }`)
	if !AppliesModelActionPolicy(e) {
		t.Fatal("an action_policy-only model override was dropped during load")
	}
}

func TestApplyModelActionPolicyLeavesDefaultPromptUntouched(t *testing.T) {
	const base = "BASE PROMPT"
	if got := ApplyModelActionPolicy(base, &ProviderEntry{}); got != base {
		t.Fatalf("default entry changed the prompt: %q", got)
	}
	if got := ApplyModelActionPolicy(base, nil); got != base {
		t.Fatalf("nil entry changed the prompt: %q", got)
	}
	opted := true
	e := &ProviderEntry{Model: "x", ModelOverrides: map[string]ProviderModelOverride{"x": {ActionPolicy: &opted}}}
	got := ApplyModelActionPolicy(base, e)
	if got != base+"\n\n"+ModelActionPolicy {
		t.Fatalf("opt-in did not append exactly one paragraph: %q", got)
	}
}
