package config

// The [[providers]] model_overrides entry and the resolution that folds the
// selected model's entry onto the provider. Split from config.go so the
// per-model keys live beside their renderer, not inside the whole-config type.

import "strings"

type ProviderModelOverride struct {
	// ReasoningDefaults identifies generated compatibility fields. A changed
	// snapshot is treated as user-owned, including edits by older writers.
	ReasoningDefaults string   `toml:"reasoning_defaults,omitempty"`
	ReasoningProtocol string   `toml:"reasoning_protocol"`
	SupportedEfforts  []string `toml:"supported_efforts"`
	DefaultEffort     string   `toml:"default_effort"`
	Vision            *bool    `toml:"vision"`
	// ContextWindow overrides the provider-wide context budget for this model.
	// Zero inherits ProviderEntry.ContextWindow so existing configurations keep
	// their current compaction behavior.
	ContextWindow int `toml:"context_window"`
	// MaxOutputTokens overrides the provider-wide output budget. Zero inherits;
	// positive values set a cap and negative values omit optional wire limits.
	MaxOutputTokens int `toml:"max_output_tokens"`
	// ActionPolicy appends ModelActionPolicy to the system prompt for this
	// model. Nil leaves it off: the paragraph costs prompt tokens on every
	// turn, and only models that narrate unperformed work need it.
	ActionPolicy *bool `toml:"action_policy"`
}

func (e *ProviderEntry) applyModelOverride() {
	if e == nil || len(e.ModelOverrides) == 0 {
		return
	}
	ov, ok := e.modelOverrideForModel(e.Model)
	if !ok {
		return
	}
	if ov.ReasoningProtocol != "" {
		e.reasoningProtocolAutomatic = false
		e.ReasoningProtocol = ov.ReasoningProtocol
	}
	if ov.SupportedEfforts != nil {
		e.reasoningAutomatic = false
		e.SupportedEfforts = append([]string(nil), ov.SupportedEfforts...)
	}
	if ov.DefaultEffort != "" || ov.SupportedEfforts != nil {
		e.reasoningDefaultAutomatic = false
		e.DefaultEffort = ov.DefaultEffort
	}
	if ov.Vision != nil {
		e.visionOverride = ov.Vision
	}
	if ov.ContextWindow > 0 {
		e.ContextWindow = ov.ContextWindow
	}
	if ov.MaxOutputTokens != 0 {
		e.MaxOutputTokens = ov.MaxOutputTokens
	}
}

func (e *ProviderEntry) modelOverrideForModel(model string) (ProviderModelOverride, bool) {
	model = strings.TrimSpace(model)
	if e == nil || model == "" || len(e.ModelOverrides) == 0 {
		return ProviderModelOverride{}, false
	}
	if ov, ok := e.ModelOverrides[model]; ok {
		return explicitModelReasoning(ov), true
	}
	return ProviderModelOverride{}, false
}
