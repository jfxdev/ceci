package oidc

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func TestOIDCService_BeginCreatesSignedPKCETransaction(t *testing.T) {
	svc := &Service{signKey: []byte("test-signing-key"), oauth: oauth2.Config{ClientID: "client", RedirectURL: "http://localhost/callback", Endpoint: oauth2.Endpoint{AuthURL: "https://idp.example.com/authorize"}}}
	authorizationURL, encoded, err := svc.Begin()
	require.NoError(t, err)
	parsed, err := url.Parse(authorizationURL)
	require.NoError(t, err)
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	assert.NotEmpty(t, parsed.Query().Get("code_challenge"))
	transaction, err := svc.decodeTransaction(encoded)
	require.NoError(t, err)
	assert.Equal(t, parsed.Query().Get("state"), transaction.State)
	assert.NotEmpty(t, transaction.Nonce)
	assert.NotEmpty(t, transaction.Verifier)
	assert.Greater(t, transaction.Expires, time.Now().Unix())
}

func TestOIDCService_RejectsTamperedOrExpiredTransaction(t *testing.T) {
	svc := &Service{signKey: []byte("test-signing-key")}
	encoded, err := svc.encodeTransaction(oidcTransaction{State: "state", Expires: time.Now().Add(-time.Minute).Unix()})
	require.NoError(t, err)
	_, err = svc.Complete(t.Context(), "state", encoded, "code")
	assert.Error(t, err)
	_, err = svc.decodeTransaction(encoded + "tampered")
	assert.Error(t, err)
}
