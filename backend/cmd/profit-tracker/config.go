package main

import (
	"flag"
	"fmt"
	"path/filepath"
)

// Environment variables that set a value when the flag is not given.
const (
	envAddr = "PROFIT_TRACKER_ADDR"
	envDB   = "PROFIT_TRACKER_DB"
)

// defaultAddr is the listen address. The Vite dev proxy sends /api requests to it.
const defaultAddr = "127.0.0.1:8080"

// config is the start configuration of the program.
type config struct {
	addr   string
	dbPath string
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
	if err := flags.Parse(args); err != nil {
		return config{}, fmt.Errorf("parse flags: %w", err)
	}

	cfg.addr = firstSet(cfg.addr, getenv(envAddr), defaultAddr)
	cfg.dbPath = firstSet(cfg.dbPath, getenv(envDB))
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
