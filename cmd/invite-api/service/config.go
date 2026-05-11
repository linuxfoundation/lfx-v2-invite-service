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
	SMTP       SMTPConfig
	LFXBaseURL string
}

// SMTPConfig holds SMTP connection parameters.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

// AppConfigFromEnv reads AppConfig from environment variables, applying defaults where needed.
func AppConfigFromEnv() AppConfig {
	natsURL := os.Getenv(constants.NATSURLEnvKey)
	if natsURL == "" {
		natsURL = "nats://lfx-platform-nats.lfx.svc.cluster.local:4222"
	}

	smtpHost := os.Getenv(constants.EmailSMTPHostEnvKey)
	if smtpHost == "" {
		smtpHost = "lfx-platform-mailpit-smtp.lfx.svc.cluster.local"
	}

	smtpPort := os.Getenv(constants.EmailSMTPPortEnvKey)
	if smtpPort == "" {
		smtpPort = "25"
	}

	lfxBaseURL := os.Getenv(constants.LFXBaseURLEnvKey)
	if lfxBaseURL == "" {
		lfxBaseURL = "https://lfx.linuxfoundation.org"
	}

	return AppConfig{
		NATSURL: natsURL,
		SMTP: SMTPConfig{
			Host:     smtpHost,
			Port:     smtpPort,
			Username: os.Getenv(constants.EmailSMTPUsernameEnvKey),
			Password: os.Getenv(constants.EmailSMTPPasswordEnvKey),
		},
		LFXBaseURL: lfxBaseURL,
	}
}
