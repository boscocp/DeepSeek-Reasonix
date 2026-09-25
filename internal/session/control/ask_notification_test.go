package control

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"reasonix/internal/base/testenv"
	"reasonix/internal/contract/event"
	"reasonix/internal/ext/hook"
)

// A question blocks the run on the user exactly like an approval prompt, so it
// fires the same Notification hook; otherwise an external channel hears about
// every pending approval while a blocking question looks like the agent working.
func TestAskFiresNotificationHook(t *testing.T) {
	var mu sync.Mutex
	var stdins []string
	hooks := hook.NewRunner([]hook.ResolvedHook{{
		HookConfig: hook.HookConfig{Command: "record-notification"},
		Event:      hook.Notification,
	}}, "", func(_ context.Context, in hook.SpawnInput) hook.SpawnResult {
		mu.Lock()
		stdins = append(stdins, in.Stdin)
		mu.Unlock()
		return hook.SpawnResult{ExitCode: 0}
	}, nil)
	c := New(Options{Sink: &askProbeSink{}, SessionDir: testenv.TempDir(t), Hooks: hooks})

	go func() { _, _ = c.Ask(t.Context(), askProbeQuestions()) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := len(stdins)
		mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a blocking question never fired the Notification hook")
		}
		time.Sleep(5 * time.Millisecond)
	}

	var payload struct {
		Event            string `json:"event"`
		Message          string `json:"message"`
		NotificationType string `json:"notificationType"`
	}
	mu.Lock()
	stdin := stdins[0]
	mu.Unlock()
	if err := json.Unmarshal([]byte(stdin), &payload); err != nil {
		t.Fatalf("hook stdin is not JSON: %v", err)
	}
	if payload.Event != string(hook.Notification) {
		t.Fatalf("event = %q, want Notification", payload.Event)
	}
	if payload.NotificationType != "question_prompt" {
		t.Fatalf("notificationType = %q, want question_prompt", payload.NotificationType)
	}
	if payload.Message != "answer needed: Which fix?" {
		t.Fatalf("message = %q, want the first question's prompt", payload.Message)
	}
}

func TestAskNotificationText(t *testing.T) {
	if got := askNotificationText(nil); got != "answer needed: " {
		t.Fatalf("no questions = %q", got)
	}
	two := []event.AskQuestion{{Prompt: "Which fix?"}, {Prompt: "Which branch?"}}
	if got := askNotificationText(two); got != "answer needed: Which fix?" {
		t.Fatalf("two questions = %q, want the first prompt", got)
	}
}
