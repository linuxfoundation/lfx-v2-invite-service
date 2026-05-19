// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import "os"

// AppConfig holds all runtime configuration read from environment variables.
type AppConfig struct {
	NATSURL    string
	LFXBaseURL string
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

	return AppConfig{
		NATSURL:    natsURL,
		LFXBaseURL: lfxBaseURL,
	}
}
