package oauth2

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStoredClientVerifyPassword(t *testing.T) {
	sum := sha256.Sum256([]byte("correct-secret"))
	hashed := "sha256:" + base64.RawURLEncoding.EncodeToString(sum[:])

	hashedClient := &storedClient{storedSecret: hashed}
	require.True(t, hashedClient.VerifyPassword("correct-secret"))
	require.False(t, hashedClient.VerifyPassword("incorrect-secret"))

	legacyClient := &storedClient{storedSecret: "legacy-secret"}
	require.True(t, legacyClient.VerifyPassword("legacy-secret"))
	require.False(t, legacyClient.VerifyPassword("incorrect-secret"))

	publicClient := &storedClient{}
	require.True(t, publicClient.VerifyPassword(""))
	require.True(t, publicClient.VerifyPassword("ignored-for-compatibility"))
}
