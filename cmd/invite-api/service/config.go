// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import "os"

// AppConfig holds all runtime configuration read from environment variables.
type AppConfig struct {
	NATSURL            string
	DefaultReturnURL         string
	InviteJWTSecret    string
	SelfServeBaseURL   string
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

	return AppConfig{
		NATSURL:          natsURL,
		DefaultReturnURL: defaultReturnURL,
		InviteJWTSecret:  os.Getenv("INVITE_JWT_SECRET"),
		SelfServeBaseURL: selfServeBaseURL,
	}
}
