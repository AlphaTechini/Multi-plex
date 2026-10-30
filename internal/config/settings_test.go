package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaultSettings(t *testing.T) {
	s := DefaultSettings()
	checks := []struct {
		name      string
		got, want any
	}{
		{"Theme", s.Theme, "dark-plus"},
		{"PrefixKey", s.PrefixKey, "ctrl+space"},
		{"AgentPanelWidth", s.AgentPanelWidth, 0.30},
		{"AgentPanelMinCols", s.AgentPanelMinCols, 60},
		{"ReduceMotion", s.ReduceMotion, false},
		{"DefaultAgent", s.DefaultAgent, "claude-code"},
		{"DefaultRole", s.DefaultRole, "implementer"},
		{"ConflictResolution", s.ConflictResolution, "ask"},
		{"Approval", s.Approval, "auto"},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestLoadSettings(t *testing.T) {
	tests := []struct {
		name    string
		file    *string // nil means the file does not exist
		check   func(t *testing.T, s Settings)
		wantErr bool
	}{
		{
			name: "missing file gives defaults",
			file: nil,
			check: func(t *testing.T, s Settings) {
				if !reflect.DeepEqual(s, DefaultSettings()) {
					t.Errorf("got %+v, want defaults", s)
				}
			},
		},
		{
			name: "empty file gives defaults",
			file: strPtr("  \n"),
			check: func(t *testing.T, s Settings) {
				if s.Theme != "dark-plus" {
					t.Errorf("Theme = %q", s.Theme)
				}
			},
		},
		{
			name: "partial file merges onto defaults",
			file: strPtr(`{"theme":"light","reduceMotion":true,"keymap":{"quit":"ctrl+q"}}`),
			check: func(t *testing.T, s Settings) {
				if s.Theme != "light" || !s.ReduceMotion {
					t.Errorf("explicit keys not applied: %+v", s)
				}
				if s.AgentPanelWidth != 0.30 || s.DefaultAgent != "claude-code" {
					t.Errorf("absent keys lost their defaults: %+v", s)
				}
				if s.Keymap["quit"] != "ctrl+q" {
					t.Errorf("Keymap = %v", s.Keymap)
				}
			},
		},
		{
			name: "unknown keys are ignored",
			file: strPtr(`{"futureThing":{"a":1},"approval":"typedcode"}`),
			check: func(t *testing.T, s Settings) {
				if s.Approval != "typedcode" {
					t.Errorf("Approval = %q", s.Approval)
				}
			},
		},
		{
			name:    "malformed json is an error",
			file:    strPtr(`{"theme":`),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			if tt.file != nil {
				if err := os.WriteFile(path, []byte(*tt.file), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			s, err := LoadSettings(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.check != nil {
				tt.check(t, s)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
