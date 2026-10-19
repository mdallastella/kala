// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the server configuration.
type Config struct {
	// Addr is the listen address, e.g. ":8080".
	Addr string
	// DBPath is the SQLite database file path.
	DBPath string
	// JWTSecret signs access tokens. When empty, a secret is generated once and
	// persisted in the database.
	JWTSecret string
	// AllowSignup enables registration after the first user exists.
	AllowSignup bool
	// Dev relaxes cookie security for plain-HTTP local development.
	Dev bool
	// TrustProxy makes the server use X-Forwarded-For for client IPs.
	TrustProxy bool
}

// Load reads the configuration from the environment.
func Load() (Config, error) {
	c := Config{
		Addr:      env("KALA_ADDR", ":8080"),
		DBPath:    env("KALA_DB_PATH", "kala.db"),
		JWTSecret: os.Getenv("KALA_JWT_SECRET"),
	}
	var err error
	if c.AllowSignup, err = envBool("KALA_ALLOW_SIGNUP"); err != nil {
		return c, err
	}
	if c.Dev, err = envBool("KALA_DEV"); err != nil {
		return c, err
	}
	if c.TrustProxy, err = envBool("KALA_TRUST_PROXY"); err != nil {
		return c, err
	}
	if c.JWTSecret != "" && len(c.JWTSecret) < 32 {
		return c, fmt.Errorf("KALA_JWT_SECRET must be at least 32 characters")
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}
