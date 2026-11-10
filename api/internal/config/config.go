// Package config loads runtime configuration from the environment.
package config

import (
	"fmt"
	"net/url"
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
	Env                 string
	SecureCookies       bool
	TOTPEncryptKey      []byte // 32 bytes, encrypts totp_secret at rest
	OpenRouterKey       string // loaded from a mounted secret file, never sent to clients
	OpenRouterModel     string
	StripeKey           string
	StripePrice         string
	StripeCoupon        string
	StripeWebhookSecret string
	PublicURL           string
	ResendKey           string
	EmailFrom           string
	SignupNotify        string
}

func Load() (*Config, error) {
	c := &Config{
		Addr:            env("BW_ADDR", ":8080"),
		DatabaseURL:     env("BW_DATABASE_URL", ""),
		Env:             env("BW_ENV", "development"),
		OpenRouterModel: env("BW_OPENROUTER_MODEL", "google/gemini-3.8-flash"),
		AllowOrigins: strings.Split(
			env("BW_ALLOW_ORIGINS", "http://localhost:4321"), ","),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("config: BW_DATABASE_URL is required")
	}

	if file := os.Getenv("BW_OPENROUTER_KEY_FILE"); file != "" {
		key, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("config: could not read OpenRouter secret file")
		}
		c.OpenRouterKey = strings.TrimSpace(string(key))
		if c.OpenRouterKey == "" {
			return nil, fmt.Errorf("config: OpenRouter secret file is empty")
		}
	}

	c.EmailFrom = env("BW_EMAIL_FROM", "Potpuri <noreply@potpuri.cc>")
	c.SignupNotify = env("BW_SIGNUP_NOTIFY", "thepandras@gmail.com")
	if file := os.Getenv("BW_RESEND_KEY_FILE"); file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("config: could not read Resend secret file")
		}
		c.ResendKey = strings.TrimSpace(string(data))
		if c.ResendKey == "" {
			return nil, fmt.Errorf("config: empty Resend key")
		}
	}
	c.StripePrice = env("BW_STRIPE_PRICE", "")
	c.StripeCoupon = env("BW_STRIPE_COUPON", "")
	c.PublicURL = env("BW_PUBLIC_URL", "http://localhost:4321")
	if c.ResendKey != "" {
		u, err := url.Parse(c.PublicURL)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && (c.Env == "production" || u.Scheme != "http")) {
			return nil, fmt.Errorf("config: email requires a valid public URL (HTTPS in production)")
		}
	}
	for _, item := range []struct {
		env    string
		target *string
	}{{"BW_STRIPE_KEY_FILE", &c.StripeKey}, {"BW_STRIPE_WEBHOOK_FILE", &c.StripeWebhookSecret}} {
		if file := os.Getenv(item.env); file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("config: could not read %s", item.env)
			}
			*item.target = strings.TrimSpace(string(data))
			if *item.target == "" {
				return nil, fmt.Errorf("config: empty %s", item.env)
			}
		}
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
