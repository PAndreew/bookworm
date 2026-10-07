// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr         string
	DatabaseURL  string
	AllowOrigins []string
	// SecureCookies must be true in production. It is derived from Env so that a
	// deployment cannot accidentally ship non-Secure session cookies.
	Env            string
	SecureCookies  bool
	TOTPEncryptKey []byte // 32 bytes, encrypts totp_secret at rest
}

func Load() (*Config, error) {
	c := &Config{
		Addr:        env("BW_ADDR", ":8080"),
		DatabaseURL: env("BW_DATABASE_URL", ""),
		Env:         env("BW_ENV", "development"),
		AllowOrigins: strings.Split(
			env("BW_ALLOW_ORIGINS", "http://localhost:4321"), ","),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("config: BW_DATABASE_URL is required")
	}

	c.SecureCookies = c.Env == "production"
	if v := os.Getenv("BW_SECURE_COOKIES"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("config: BW_SECURE_COOKIES: %w", err)
		}
		// Allow forcing on, never off in production.
		c.SecureCookies = b || c.Env == "production"
	}

	key := env("BW_TOTP_KEY", "")
	switch {
	case key == "" && c.Env == "production":
		return nil, fmt.Errorf("config: BW_TOTP_KEY is required in production")
	case key == "":
		// Development only: a fixed key so restarts do not invalidate enrolments.
		key = strings.Repeat("d", 32)
	case len(key) != 32:
		return nil, fmt.Errorf("config: BW_TOTP_KEY must be exactly 32 bytes, got %d", len(key))
	}
	c.TOTPEncryptKey = []byte(key)

	for i, o := range c.AllowOrigins {
		c.AllowOrigins[i] = strings.TrimSpace(o)
	}
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
