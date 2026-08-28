// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package assistant

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRequiredRole = "abera-ai-user"
	defaultTimeout      = 95 * time.Second
	maxJSONBody         = 1 << 20
	maxUploadBody       = 10 << 20
)

var tenantPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Config struct {
	Enabled          bool
	AgentURL         *url.URL
	TenantID         string
	RequiredRole     string
	AuthMode         string
	APIKey           string
	LocalToken       string
	AWSRegion        string
	Timeout          time.Duration
	MaxJSONBody      int64
	MaxUploadBody    int64
	PublicBridgePath string
}

func ConfigFromEnvironment() (Config, error) {
	cfg := Config{
		RequiredRole:     defaultRequiredRole,
		AuthMode:         "aws_iam",
		Timeout:          defaultTimeout,
		MaxJSONBody:      maxJSONBody,
		MaxUploadBody:    maxUploadBody,
		PublicBridgePath: "/api/system/assistant",
	}

	if raw := strings.TrimSpace(os.Getenv("ABERA_AI_ENABLED")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			return cfg, fmt.Errorf("ABERA_AI_ENABLED must be true or false: %w", err)
		}
		cfg.Enabled = enabled
	}
	if !cfg.Enabled {
		return cfg, nil
	}

	rawURL := strings.TrimSpace(os.Getenv("ABERA_AI_AGENT_URL"))
	if rawURL == "" {
		return cfg, fmt.Errorf("ABERA_AI_AGENT_URL is required when ABERA_AI_ENABLED=true")
	}
	agentURL, err := url.Parse(rawURL)
	if err != nil || agentURL.Host == "" || (agentURL.Scheme != "http" && agentURL.Scheme != "https") {
		return cfg, fmt.Errorf("ABERA_AI_AGENT_URL must be an absolute HTTP(S) URL")
	}
	if agentURL.User != nil || agentURL.RawQuery != "" || agentURL.Fragment != "" {
		return cfg, fmt.Errorf("ABERA_AI_AGENT_URL must not contain credentials, a query, or a fragment")
	}
	cfg.AgentURL = agentURL

	cfg.TenantID = strings.TrimSpace(os.Getenv("ABERA_AI_TENANT_ID"))
	if !tenantPattern.MatchString(cfg.TenantID) {
		return cfg, fmt.Errorf("ABERA_AI_TENANT_ID must match %s", tenantPattern.String())
	}
	if value := strings.TrimSpace(os.Getenv("ABERA_AI_REQUIRED_ROLE")); value != "" {
		cfg.RequiredRole = value
	}
	if !tenantPattern.MatchString(cfg.RequiredRole) {
		return cfg, fmt.Errorf("ABERA_AI_REQUIRED_ROLE must be a valid role handle")
	}
	if value := strings.TrimSpace(os.Getenv("ABERA_AI_AUTH_MODE")); value != "" {
		cfg.AuthMode = value
	}

	if value := strings.TrimSpace(os.Getenv("ABERA_AI_TIMEOUT")); value != "" {
		duration, err := time.ParseDuration(value)
		if err != nil || duration < time.Second || duration > 2*time.Minute {
			return cfg, fmt.Errorf("ABERA_AI_TIMEOUT must be between 1s and 2m")
		}
		cfg.Timeout = duration
	}

	if path := strings.TrimSpace(os.Getenv("ABERA_AI_API_KEY_FILE")); path != "" {
		cfg.APIKey, err = readSecretFile(path, 8)
		if err != nil {
			return cfg, fmt.Errorf("ABERA_AI_API_KEY_FILE: %w", err)
		}
	}

	switch cfg.AuthMode {
	case "aws_iam":
		if cfg.APIKey == "" {
			return cfg, fmt.Errorf("ABERA_AI_API_KEY_FILE is required in aws_iam mode")
		}
		cfg.AWSRegion = firstNonEmpty(
			os.Getenv("ABERA_AI_AWS_REGION"),
			os.Getenv("AWS_REGION"),
			os.Getenv("AWS_DEFAULT_REGION"),
		)
		if cfg.AWSRegion == "" {
			return cfg, fmt.Errorf("ABERA_AI_AWS_REGION or AWS_REGION is required in aws_iam mode")
		}
	case "local_token":
		path := strings.TrimSpace(os.Getenv("ABERA_AI_LOCAL_TOKEN_FILE"))
		if path == "" {
			return cfg, fmt.Errorf("ABERA_AI_LOCAL_TOKEN_FILE is required in local_token mode")
		}
		cfg.LocalToken, err = readSecretFile(path, 32)
		if err != nil {
			return cfg, fmt.Errorf("ABERA_AI_LOCAL_TOKEN_FILE: %w", err)
		}
	default:
		return cfg, fmt.Errorf("ABERA_AI_AUTH_MODE must be aws_iam or local_token")
	}

	return cfg, nil
}

func readSecretFile(path string, minimumLength int) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path must reference a regular file and not a symbolic link")
	}
	if info.Size() > 16*1024 {
		return "", fmt.Errorf("secret file is unexpectedly large")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(contents))
	if len(value) < minimumLength {
		return "", fmt.Errorf("secret must contain at least %d characters", minimumLength)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("secret must contain exactly one line")
	}
	return value, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
