// Copyright The Linux Foundation and each contributor to LFX.
// SPDX-License-Identifier: MIT

package service

import (
	"os"

	"github.com/linuxfoundation/lfx-v2-invite-service/pkg/constants"
)

// AppConfig holds all runtime configuration read from environment variables.
type AppConfig struct {
	NATSURL    string
	LFXBaseURL string
}

// AppConfigFromEnv reads AppConfig from environment variables, applying defaults where needed.
func AppConfigFromEnv() AppConfig {
	natsURL := os.Getenv(constants.NATSURLEnvKey)
	if natsURL == "" {
		natsURL = "nats://lfx-platform-nats.lfx.svc.cluster.local:4222"
	}

	lfxBaseURL := os.Getenv(constants.LFXBaseURLEnvKey)
	if lfxBaseURL == "" {
		lfxBaseURL = "https://lfx.linuxfoundation.org"
	}

	return AppConfig{
		NATSURL:    natsURL,
		LFXBaseURL: lfxBaseURL,
	}
}
