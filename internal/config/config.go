package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct{ DatabaseURL, EncryptionKey, CronSecret, Origin, Port, SetupCode string }

// DemoMode matches the frontend build flag. Demo deployments never open a database.
func DemoMode() bool { return os.Getenv("NEXT_PUBLIC_DEMO_MODE") == "true" }

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), CronSecret: os.Getenv("CRON_SECRET"), Origin: strings.TrimSuffix(os.Getenv("APP_ORIGIN"), "/"), Port: os.Getenv("PORT"), SetupCode: os.Getenv("SETUP_CODE")}
	if c.SetupCode != "" && (len(c.SetupCode) != 4 || strings.Trim(c.SetupCode, "0123456789") != "") {
		return c, errors.New("SETUP_CODE must contain exactly four digits")
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.DatabaseURL == "" || c.EncryptionKey == "" || len(c.CronSecret) < 32 {
		return c, errors.New("DATABASE_URL, ENCRYPTION_KEY and CRON_SECRET (at least 32 bytes) are required")
	}
	u, e := url.Parse(c.Origin)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) {
		return c, errors.New("APP_ORIGIN must be an HTTPS origin (HTTP localhost allowed for development)")
	}
	return c, nil
}
