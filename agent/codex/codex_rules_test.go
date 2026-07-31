package codex

import (
	"testing"

	"github.com/chenhg5/cc-connect/core"
)

func TestAgentResolveMode(t *testing.T) {
	a := &Agent{
		modeRules: map[string]string{
			"qq:123456789": "full-auto",
			"qq:987654321": "auto-edit",
			"qq:*":         "suggest",
		},
	}

	tests := []struct {
		name       string
		sessionKey string
		want       string
	}{
		{name: "exact private admin", sessionKey: "qq:123456789", want: "full-auto"},
		{name: "group prefix rule", sessionKey: "qq:987654321:123456789", want: "auto-edit"},
		{name: "shared group exact", sessionKey: "qq:g:987654321", want: ""},
		{name: "wildcard fallback", sessionKey: "qq:123456", want: "suggest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.ResolveMode(&core.Message{SessionKey: tt.sessionKey}); got != tt.want {
				t.Fatalf("ResolveMode(%q) = %q, want %q", tt.sessionKey, got, tt.want)
			}
		})
	}
}

func TestAgentResolveModeNoRules(t *testing.T) {
	a := &Agent{}
	if got := a.ResolveMode(&core.Message{SessionKey: "qq:1"}); got != "" {
		t.Fatalf("no rules: got %q, want empty (fall back to project default)", got)
	}
}

func TestParseModeRules(t *testing.T) {
	rules := parseModeRules(map[string]any{
		"qq:123456789": "full-auto",
		"qq:g:987654321": "yolo",
		"bad":          123,
	})
	if got := rules["qq:123456789"]; got != "full-auto" {
		t.Errorf("exact rule = %q, want full-auto", got)
	}
	if got := rules["qq:g:987654321"]; got != "yolo" {
		t.Errorf("group rule = %q, want yolo", got)
	}
	if _, ok := rules["bad"]; ok {
		t.Error("non-string rule should be skipped")
	}
}
