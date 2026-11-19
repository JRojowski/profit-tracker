package main

import (
	"flag"
	"fmt"
	"path/filepath"

	"github.com/jrojowski/profit-tracker/backend/internal/openrouter"
)

// Environment variables that set a value when the flag is not given.
const (
	envAddr           = "PROFIT_TRACKER_ADDR"
	envDB             = "PROFIT_TRACKER_DB"
	envAssistantModel = "PROFIT_TRACKER_ASSISTANT_MODEL"
)

// Environment variables with no flag.
const (
	// envOpenRouterKey is the API key of OpenRouter. It has no flag, because a flag is visible in the process list.
	// If it is empty, the Assistant is off.
	envOpenRouterKey = "PROFIT_TRACKER_OPENROUTER_KEY"
	// envOpenRouterURL is the base URL of the provider. It lets a local fake provider serve the manual checks.
	envOpenRouterURL = "PROFIT_TRACKER_OPENROUTER_URL"
)

// defaultAddr is the listen address. The Vite dev proxy sends /api requests to it.
const defaultAddr = "127.0.0.1:8088"

// config is the start configuration of the program.
type config struct {
	addr   string
	dbPath string
	// openRouterKey is the API key of OpenRouter, or empty if the Assistant is off. Never log it.
	openRouterKey string
	// assistantModel is the model ID of the Assistant.
	assistantModel string
	// openRouterURL is the base URL of the OpenRouter API.
	openRouterURL string
}

// parseConfig reads the configuration. A flag overrides the environment, and the environment overrides the default.
// The default database is in the user's configuration folder, for example ~/Library/Application Support on macOS.
// parseConfig looks up that folder only when neither -db nor the environment gives a database path.
func parseConfig(args []string, getenv func(string) string, userConfigDir func() (string, error)) (config, error) {
	var cfg config
	flags := flag.NewFlagSet("profit-tracker", flag.ContinueOnError)
	flags.StringVar(&cfg.addr, "addr", "", "listen address (env "+envAddr+", default "+defaultAddr+")")
	flags.StringVar(&cfg.dbPath, "db", "", "path of the SQLite database file "+
		"(env "+envDB+", default <user config folder>/profit-tracker/profit-tracker.db)")
	flags.StringVar(&cfg.assistantModel, "assistant-model", "", "OpenRouter model ID of the Assistant "+
		"(env "+envAssistantModel+", default "+openrouter.DefaultModel+")")
	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg.addr = firstSet(cfg.addr, getenv(envAddr), defaultAddr)
	cfg.dbPath = firstSet(cfg.dbPath, getenv(envDB))
	cfg.assistantModel = firstSet(cfg.assistantModel, getenv(envAssistantModel), openrouter.DefaultModel)
	cfg.openRouterKey = getenv(envOpenRouterKey)
	cfg.openRouterURL = firstSet(getenv(envOpenRouterURL), openrouter.DefaultBaseURL)
	if cfg.dbPath != "" {
		return cfg, nil
	}

	dir, err := userConfigDir()
	if err != nil {
		return config{}, fmt.Errorf("find the user configuration folder (set -db or %s): %w", envDB, err)
	}
	cfg.dbPath = filepath.Join(dir, "profit-tracker", "profit-tracker.db")
	return cfg, nil
}

// firstSet returns the first value that is not empty.
func firstSet(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
