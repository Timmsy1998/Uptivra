package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct{ DatabaseURL, EncryptionKey, CronSecret, Origin, Port string }

func Load() (Config, error) {
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), EncryptionKey: os.Getenv("ENCRYPTION_KEY"), CronSecret: os.Getenv("CRON_SECRET"), Origin: strings.TrimSuffix(os.Getenv("APP_ORIGIN"), "/"), Port: os.Getenv("PORT")}
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
