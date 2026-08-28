// Copyright 2026 Abera/Corteza contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package assistant

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type RequestSigner interface {
	Sign(context.Context, *http.Request, []byte) error
}

type BearerSigner struct {
	Token string
}

func (s BearerSigner) Sign(_ context.Context, request *http.Request, _ []byte) error {
	request.Header.Set("Authorization", "Bearer "+s.Token)
	return nil
}

type SigV4Signer struct {
	Region      string
	Credentials CredentialProvider
	Now         func() time.Time
}

func (s SigV4Signer) Sign(ctx context.Context, request *http.Request, body []byte) error {
	credentials, err := s.Credentials.Credentials(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	bodyHash := sha256Hex(body)

	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", bodyHash)
	if credentials.SessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", credentials.SessionToken)
	}

	canonicalHeaders := map[string]string{
		"host":                 request.URL.Host,
		"x-amz-content-sha256": bodyHash,
		"x-amz-date":           amzDate,
	}
	for _, name := range []string{
		"content-type",
		"x-abera-tenant-id",
		"x-abera-user-email",
		"x-abera-user-handle",
		"x-abera-user-id",
		"x-abera-user-name",
		"x-abera-user-roles",
		"x-api-key",
		"x-request-id",
	} {
		if value := request.Header.Get(name); value != "" {
			canonicalHeaders[name] = value
		}
	}
	if credentials.SessionToken != "" {
		canonicalHeaders["x-amz-security-token"] = credentials.SessionToken
	}
	keys := make([]string, 0, len(canonicalHeaders))
	for key := range canonicalHeaders {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var headerBlock strings.Builder
	for _, key := range keys {
		headerBlock.WriteString(key)
		headerBlock.WriteByte(':')
		headerBlock.WriteString(normalizeHeader(canonicalHeaders[key]))
		headerBlock.WriteByte('\n')
	}
	signedHeaders := strings.Join(keys, ";")
	canonicalURI := request.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI,
		canonicalQuery(request.URL.Query()),
		headerBlock.String(),
		signedHeaders,
		bodyHash,
	}, "\n")

	scope := fmt.Sprintf("%s/%s/execute-api/aws4_request", date, s.Region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	dateKey := hmacSHA256([]byte("AWS4"+credentials.SecretAccessKey), date)
	regionKey := hmacSHA256(dateKey, s.Region)
	serviceKey := hmacSHA256(regionKey, "execute-api")
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	request.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		credentials.AccessKeyID,
		scope,
		signedHeaders,
		signature,
	))
	return nil
}

// canonicalQuery applies the URI encoding required by AWS Signature Version 4.
// In particular, spaces must be encoded as %20 (never +), and sorting happens
// after both names and values have been encoded.
func canonicalQuery(values url.Values) string {
	type queryPair struct {
		name  string
		value string
	}
	pairs := make([]queryPair, 0, len(values))
	for name, entries := range values {
		encodedName := awsURIEncode(name)
		if len(entries) == 0 {
			pairs = append(pairs, queryPair{name: encodedName})
			continue
		}
		for _, value := range entries {
			pairs = append(pairs, queryPair{name: encodedName, value: awsURIEncode(value)})
		}
	}
	sort.Slice(pairs, func(left, right int) bool {
		if pairs[left].name == pairs[right].name {
			return pairs[left].value < pairs[right].value
		}
		return pairs[left].name < pairs[right].name
	})
	var canonical strings.Builder
	for index, pair := range pairs {
		if index > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(pair.name)
		canonical.WriteByte('=')
		canonical.WriteString(pair.value)
	}
	return canonical.String()
}

func awsURIEncode(value string) string {
	const hexadecimal = "0123456789ABCDEF"
	var encoded strings.Builder
	encoded.Grow(len(value))
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '.' || character == '_' || character == '~' {
			encoded.WriteByte(character)
			continue
		}
		encoded.WriteByte('%')
		encoded.WriteByte(hexadecimal[character>>4])
		encoded.WriteByte(hexadecimal[character&0x0f])
	}
	return encoded.String()
}

func normalizeHeader(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	digest := hmac.New(sha256.New, key)
	_, _ = digest.Write([]byte(value))
	return digest.Sum(nil)
}
