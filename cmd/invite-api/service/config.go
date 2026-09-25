// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"os"
	"strings"
)

// AppConfig holds all runtime configuration read from environment variables.
type AppConfig struct {
	NATSURL               string
	LogLevel              string
	DefaultReturnURL      string
	InviteJWTSecret       string
	SelfServeBaseURL      string
	AllowedReturnURLHosts []string
	// InvitesKVBucket is the name of the NATS JetStream KeyValue bucket used
	// to store invite records. The bucket must already exist.
	InvitesKVBucket string
}

// AppConfigFromEnv reads AppConfig from environment variables, applying defaults where needed.
func AppConfigFromEnv() AppConfig {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://lfx-platform-nats.lfx.svc.cluster.local:4222"
	}

	selfServeBaseURL := os.Getenv("LFX_SELF_SERVE_BASE_URL")
	if selfServeBaseURL == "" {
		switch os.Getenv("LFX_ENVIRONMENT") {
		case "prod":
			selfServeBaseURL = "https://app.lfx.dev"
		case "staging", "stg":
			selfServeBaseURL = "https://app.staging.lfx.dev"
		default:
			selfServeBaseURL = "https://app.dev.lfx.dev"
		}
	}

	defaultReturnURL := os.Getenv("DEFAULT_INVITE_LINK_RETURN_URL")
	if defaultReturnURL == "" {
		defaultReturnURL = selfServeBaseURL
	}

	allowedHosts := parseAllowedReturnURLHosts(os.Getenv("ALLOWED_RETURN_URL_HOSTS"))

	invitesKVBucket := os.Getenv("INVITES_KV_BUCKET")
	if invitesKVBucket == "" {
		invitesKVBucket = "invites"
	}

	return AppConfig{
		NATSURL:               natsURL,
		LogLevel:              os.Getenv("LOG_LEVEL"),
		DefaultReturnURL:      defaultReturnURL,
		InviteJWTSecret:       os.Getenv("INVITE_JWT_SECRET"),
		SelfServeBaseURL:      selfServeBaseURL,
		AllowedReturnURLHosts: allowedHosts,
		InvitesKVBucket:       invitesKVBucket,
	}
}

// parseAllowedReturnURLHosts splits a comma-separated list of host patterns,
// defaulting to the LFX-owned domains when the env var is unset.
func parseAllowedReturnURLHosts(raw string) []string {
	if raw == "" {
		return []string{"*.lfx.dev", "*.linuxfoundation.org"}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if h := strings.TrimSpace(p); h != "" {
			out = append(out, h)
		}
	}
	return out
}
