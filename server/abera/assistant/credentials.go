// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

type CredentialProvider interface {
	Credentials(context.Context) (AWSCredentials, error)
}

type DefaultCredentialProvider struct {
	client *http.Client
	mu     sync.Mutex
	cached AWSCredentials
}

func NewDefaultCredentialProvider() *DefaultCredentialProvider {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Metadata and container credential endpoints are local. Never send these
	// requests through a user-configured HTTP proxy.
	transport.Proxy = nil
	return &DefaultCredentialProvider{
		client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (p *DefaultCredentialProvider) Credentials(ctx context.Context) (AWSCredentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached.AccessKeyID != "" && (p.cached.Expiration.IsZero() || time.Until(p.cached.Expiration) > 5*time.Minute) {
		return p.cached, nil
	}

	credentials, err := p.load(ctx)
	if err != nil {
		return AWSCredentials{}, err
	}
	if credentials.AccessKeyID == "" || credentials.SecretAccessKey == "" {
		return AWSCredentials{}, fmt.Errorf("AWS credentials are incomplete")
	}
	p.cached = credentials
	return credentials, nil
}

func (p *DefaultCredentialProvider) load(ctx context.Context) (AWSCredentials, error) {
	accessKey := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if accessKey != "" || secretKey != "" {
		if accessKey == "" || secretKey == "" {
			return AWSCredentials{}, fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY must be set together")
		}
		return AWSCredentials{
			AccessKeyID:     accessKey,
			SecretAccessKey: secretKey,
			SessionToken:    strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN")),
		}, nil
	}

	if relative := strings.TrimSpace(os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI")); relative != "" {
		if !strings.HasPrefix(relative, "/") || strings.Contains(relative, "..") {
			return AWSCredentials{}, fmt.Errorf("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI is invalid")
		}
		return p.loadContainer(ctx, "http://169.254.170.2"+relative)
	}
	if full := strings.TrimSpace(os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")); full != "" {
		if err := validateContainerCredentialURL(full); err != nil {
			return AWSCredentials{}, err
		}
		return p.loadContainer(ctx, full)
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("AWS_EC2_METADATA_DISABLED")), "true") {
		return AWSCredentials{}, fmt.Errorf("no AWS credentials found and EC2 metadata is disabled")
	}
	return p.loadIMDSv2(ctx)
}

func validateContainerCredentialURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil {
		return fmt.Errorf("AWS_CONTAINER_CREDENTIALS_FULL_URI must be a safe local HTTP URL")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	for _, allowed := range []string{"169.254.170.2", "169.254.170.23"} {
		if ip != nil && ip.Equal(net.ParseIP(allowed)) {
			return nil
		}
	}
	return fmt.Errorf("AWS_CONTAINER_CREDENTIALS_FULL_URI host must be loopback or an AWS container credential endpoint")
}

type metadataCredentials struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	Token           string `json:"Token"`
	Expiration      string `json:"Expiration"`
	Code            string `json:"Code"`
}

func (p *DefaultCredentialProvider) loadContainer(ctx context.Context, endpoint string) (AWSCredentials, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return AWSCredentials{}, err
	}
	authorization := strings.TrimSpace(os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN"))
	if path := strings.TrimSpace(os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE")); path != "" {
		authorization, err = readSecretFile(path, 1)
		if err != nil {
			return AWSCredentials{}, fmt.Errorf("AWS container authorization token: %w", err)
		}
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	var payload metadataCredentials
	if err = p.fetchJSON(req, &payload); err != nil {
		return AWSCredentials{}, fmt.Errorf("load ECS credentials: %w", err)
	}
	return metadataPayload(payload)
}

func (p *DefaultCredentialProvider) loadIMDSv2(ctx context.Context) (AWSCredentials, error) {
	const endpoint = "http://169.254.169.254/latest"
	tokenRequest, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint+"/api/token", nil)
	if err != nil {
		return AWSCredentials{}, err
	}
	tokenRequest.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")
	token, err := p.fetchText(tokenRequest)
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("load IMDSv2 token: %w", err)
	}

	roleRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/meta-data/iam/security-credentials/", nil)
	if err != nil {
		return AWSCredentials{}, err
	}
	roleRequest.Header.Set("X-aws-ec2-metadata-token", token)
	role, err := p.fetchText(roleRequest)
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("load IAM role name: %w", err)
	}
	role = strings.TrimSpace(strings.Split(role, "\n")[0])
	if role == "" || strings.ContainsAny(role, "/\\") || strings.Contains(role, "..") {
		return AWSCredentials{}, fmt.Errorf("IMDS returned an invalid IAM role name")
	}

	credentialsRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/meta-data/iam/security-credentials/"+url.PathEscape(role), nil)
	if err != nil {
		return AWSCredentials{}, err
	}
	credentialsRequest.Header.Set("X-aws-ec2-metadata-token", token)
	var payload metadataCredentials
	if err = p.fetchJSON(credentialsRequest, &payload); err != nil {
		return AWSCredentials{}, fmt.Errorf("load EC2 credentials: %w", err)
	}
	return metadataPayload(payload)
}

func metadataPayload(payload metadataCredentials) (AWSCredentials, error) {
	if payload.Code != "" && payload.Code != "Success" {
		return AWSCredentials{}, fmt.Errorf("metadata credential provider returned %s", payload.Code)
	}
	expires, err := time.Parse(time.RFC3339, payload.Expiration)
	if err != nil {
		return AWSCredentials{}, fmt.Errorf("metadata credential expiration is invalid: %w", err)
	}
	return AWSCredentials{
		AccessKeyID:     payload.AccessKeyID,
		SecretAccessKey: payload.SecretAccessKey,
		SessionToken:    payload.Token,
		Expiration:      expires,
	}, nil
}

func (p *DefaultCredentialProvider) fetchText(req *http.Request) (string, error) {
	response, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("metadata endpoint returned HTTP %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 16*1024+1))
	if err != nil {
		return "", err
	}
	if len(payload) > 16*1024 {
		return "", fmt.Errorf("metadata response is too large")
	}
	return strings.TrimSpace(string(payload)), nil
}

func (p *DefaultCredentialProvider) fetchJSON(req *http.Request, output any) error {
	payload, err := p.fetchText(req)
	if err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(payload), output); err != nil {
		return fmt.Errorf("invalid metadata JSON: %w", err)
	}
	return nil
}
