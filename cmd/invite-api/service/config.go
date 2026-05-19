// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import "os"

// AppConfig holds all runtime configuration read from environment variables.
type AppConfig struct {
	NATSURL            string
	LFXBaseURL         string
	InviteJWTSecret    string
	SelfServeBaseURL   string
}

// AppConfigFromEnv reads AppConfig from environment variables, applying defaults where needed.
func AppConfigFromEnv() AppConfig {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://lfx-platform-nats.lfx.svc.cluster.local:4222"
	}

	lfxBaseURL := os.Getenv("LFX_BASE_URL")
	if lfxBaseURL == "" {
		lfxBaseURL = "https://lfx.linuxfoundation.org"
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

	return AppConfig{
		NATSURL:          natsURL,
		LFXBaseURL:       lfxBaseURL,
		InviteJWTSecret:  os.Getenv("INVITE_JWT_SECRET"),
		SelfServeBaseURL: selfServeBaseURL,
	}
}
