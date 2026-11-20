package main

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	t.Parallel()

	env := func(vars map[string]string) func(string) string {
		return func(key string) string { return vars[key] }
	}
	userConfigDir := func() (string, error) { return "/home/user/.config", nil }
	noUserConfigDir := func() (string, error) { return "", errors.New("$HOME is not defined") }

	tests := []struct {
		name          string
		args          []string
		env           map[string]string
		userConfigDir func() (string, error)
		wantAddr      string
		wantDB        string
	}{
		{
			name:     "defaults",
			wantAddr: "127.0.0.1:8088",
			wantDB:   filepath.Join("/home/user/.config", "profit-tracker", "profit-tracker.db"),
		},
		{
			name:     "environment overrides defaults",
			env:      map[string]string{"PROFIT_TRACKER_ADDR": "127.0.0.1:9000", "PROFIT_TRACKER_DB": "/data/env.db"},
			wantAddr: "127.0.0.1:9000",
			wantDB:   "/data/env.db",
		},
		{
			name:     "flags override environment",
			args:     []string{"-addr", "127.0.0.1:9100", "-db", "/data/flag.db"},
			env:      map[string]string{"PROFIT_TRACKER_ADDR": "127.0.0.1:9000", "PROFIT_TRACKER_DB": "/data/env.db"},
			wantAddr: "127.0.0.1:9100",
			wantDB:   "/data/flag.db",
		},
		{
			name:          "db flag works without a user configuration folder",
			args:          []string{"-db", "/data/flag.db"},
			userConfigDir: noUserConfigDir,
			wantAddr:      "127.0.0.1:8088",
			wantDB:        "/data/flag.db",
		},
		{
			name:          "db environment works without a user configuration folder",
			env:           map[string]string{"PROFIT_TRACKER_DB": "/data/env.db"},
			userConfigDir: noUserConfigDir,
			wantAddr:      "127.0.0.1:8088",
			wantDB:        "/data/env.db",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			configDir := userConfigDir
			if tc.userConfigDir != nil {
				configDir = tc.userConfigDir
			}

			got, err := parseConfig(tc.args, env(tc.env), configDir)

			require.NoError(t, err)
			assert.Equal(t, tc.wantAddr, got.addr)
			assert.Equal(t, tc.wantDB, got.dbPath)
		})
	}
}

func TestParseConfig_NoDBAndNoUserConfigDir_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := parseConfig(nil, func(string) string { return "" }, func() (string, error) { return "", errors.New("$HOME is not defined") })

	require.Error(t, err)
	assert.Contains(t, err.Error(), "-db")
}

func TestParseConfig_Assistant(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		env       map[string]string
		wantKey   string
		wantModel string
		wantURL   string
	}{
		{
			name:      "defaults: no key, the default model and URL",
			wantModel: "nvidia/nemotron-3-ultra-550b-a55b:free",
			wantURL:   "https://openrouter.ai/api/v1",
		},
		{
			name: "environment sets the key, the model and the URL",
			env: map[string]string{
				"PROFIT_TRACKER_OPENROUTER_KEY":  "sk-or-v1-secret",
				"PROFIT_TRACKER_ASSISTANT_MODEL": "vendor/paid-model",
				"PROFIT_TRACKER_OPENROUTER_URL":  "http://127.0.0.1:9999/api/v1",
			},
			wantKey:   "sk-or-v1-secret",
			wantModel: "vendor/paid-model",
			wantURL:   "http://127.0.0.1:9999/api/v1",
		},
		{
			name:      "flag overrides the model of the environment",
			args:      []string{"-assistant-model", "vendor/flag-model"},
			env:       map[string]string{"PROFIT_TRACKER_ASSISTANT_MODEL": "vendor/env-model"},
			wantModel: "vendor/flag-model",
			wantURL:   "https://openrouter.ai/api/v1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseConfig(tc.args, func(key string) string { return tc.env[key] }, func() (string, error) { return "/home/user/.config", nil })

			require.NoError(t, err)
			assert.Equal(t, tc.wantKey, got.openRouterKey)
			assert.Equal(t, tc.wantModel, got.assistantModel)
			assert.Equal(t, tc.wantURL, got.openRouterURL)
		})
	}
}

func TestParseConfig_NoFlagForTheKey(t *testing.T) {
	t.Parallel()

	// A flag is visible in the process list. Thus the key has no flag.
	_, err := parseConfig([]string{"-openrouter-key", "sk-or-v1-secret"}, func(string) string { return "" },
		func() (string, error) { return "/home/user/.config", nil })

	require.Error(t, err)
}

func TestAssistantModel_OnlyWithAKey(t *testing.T) {
	t.Parallel()

	assert.Nil(t, assistantModel(config{assistantModel: "vendor/model", openRouterURL: "http://127.0.0.1:1"}),
		"with no key, the Assistant is off")
	assert.NotNil(t, assistantModel(config{openRouterKey: "sk-or-v1-secret", assistantModel: "vendor/model", openRouterURL: "http://127.0.0.1:1"}))
}
