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
			wantAddr: "127.0.0.1:8080",
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
			wantAddr:      "127.0.0.1:8080",
			wantDB:        "/data/flag.db",
		},
		{
			name:          "db environment works without a user configuration folder",
			env:           map[string]string{"PROFIT_TRACKER_DB": "/data/env.db"},
			userConfigDir: noUserConfigDir,
			wantAddr:      "127.0.0.1:8080",
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
