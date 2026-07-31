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
			"*":            "suggest",
		},
	}

	tests := []struct {
		name       string
		sessionKey string
		want       string
	}{
		{name: "exact private admin", sessionKey: "qq:123456789", want: "full-auto"},
		{name: "group prefix rule", sessionKey: "qq:987654321:123456789", want: "auto-edit"},
		{name: "shared group falls to wildcard", sessionKey: "qq:g:987654321", want: "suggest"},
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
		"qq:123456789":   "full-auto",
		"qq:g:987654321": "yolo",
		"bad":            123,
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

func TestAgentResolveWorkDir(t *testing.T) {
	a := &Agent{
		workDirRules: map[string]string{
			"qq:123456789": "/workspace/private",
			"qq:987654321": "/workspace/group-prefix",
			"*":            "/workspace/fallback",
		},
	}

	tests := []struct {
		name       string
		sessionKey string
		want       string
	}{
		{name: "exact private chat", sessionKey: "qq:123456789", want: "/workspace/private"},
		{name: "group per-user prefix rule", sessionKey: "qq:987654321:123456789", want: "/workspace/group-prefix"},
		{name: "shared group falls to wildcard", sessionKey: "qq:g:987654321", want: "/workspace/fallback"},
		{name: "wildcard fallback", sessionKey: "qq:123456", want: "/workspace/fallback"},
		{name: "wildcard catches other platforms", sessionKey: "slack:C123", want: "/workspace/fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := a.ResolveWorkDir(tt.sessionKey); got != tt.want {
				t.Fatalf("ResolveWorkDir(%q) = %q, want %q", tt.sessionKey, got, tt.want)
			}
		})
	}
}

func TestAgentResolveWorkDirNoWildcard(t *testing.T) {
	a := &Agent{
		workDirRules: map[string]string{
			"qq:123456789": "/workspace/private",
		},
	}
	for _, key := range []string{"qq:g:987654321", "slack:C123", "qq:999"} {
		if got := a.ResolveWorkDir(key); got != "" {
			t.Fatalf("ResolveWorkDir(%q) = %q, want empty (no rule matches)", key, got)
		}
	}
}

func TestAgentResolveWorkDirNoRules(t *testing.T) {
	a := &Agent{}
	if got := a.ResolveWorkDir("qq:1"); got != "" {
		t.Fatalf("no rules: got %q, want empty (fall back to project default work_dir)", got)
	}
}

func TestParseWorkDirRules(t *testing.T) {
	rules := parseWorkDirRules(map[string]any{
		"qq:123456789":   "/workspace/private",
		"qq:g:987654321": "/workspace/group",
		"bad":            123,
		"empty":          "",
		" whitespace ":   " /workspace/trimmed ",
	})
	if got := rules["qq:123456789"]; got != "/workspace/private" {
		t.Errorf("private rule = %q, want /workspace/private", got)
	}
	if got := rules["qq:g:987654321"]; got != "/workspace/group" {
		t.Errorf("group rule = %q, want /workspace/group", got)
	}
	if _, ok := rules["bad"]; ok {
		t.Error("non-string rule should be skipped")
	}
	if _, ok := rules["empty"]; ok {
		t.Error("empty rule should be skipped")
	}
	if got := rules["whitespace"]; got != "/workspace/trimmed" {
		t.Errorf("trimmed rule = %q, want /workspace/trimmed", got)
	}
}
