// Package oidc holds the OIDC single-sign-on domain logic — the stateless
// per-provider auth-code/PKCE flow (Service) and the persisted instance-wide
// configuration around it (ConfigurationService) — split out from
// internal/service so it can be maintained and tested independently of the
// rest of the service layer.
package oidc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const providerName = "oidc"

type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	JITEnabled   bool
}

func (c Config) Enabled() bool {
	return c.IssuerURL != "" && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != ""
}

type Service struct {
	config   Config
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	signKey  []byte
}

func (s *Service) JITEnabled() bool { return s.config.JITEnabled }

func (s *Service) FrontendURL() string {
	// The configured callback is same-origin with the SPA. Only its origin is
	// reused so the callback path can remain an API endpoint.
	parsed, err := url.Parse(s.config.RedirectURL)
	if err != nil {
		return "/projects"
	}
	return parsed.Scheme + "://" + parsed.Host + "/projects"
}

type Identity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
}

type oidcTransaction struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Expires  int64  `json:"expires"`
}

func NewService(ctx context.Context, config Config, signingSecret string) (*Service, error) {
	if !config.Enabled() {
		return nil, nil
	}
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover oidc provider: %w", err)
	}
	return &Service{
		config:   config,
		oauth:    oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, RedirectURL: config.RedirectURL, Endpoint: provider.Endpoint(), Scopes: []string{oidc.ScopeOpenID, "profile", "email"}},
		verifier: provider.Verifier(&oidc.Config{ClientID: config.ClientID}),
		signKey:  []byte(signingSecret),
	}, nil
}

func secureRandomString(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func (s *Service) Begin() (string, string, error) {
	state, err := secureRandomString(32)
	if err != nil {
		return "", "", err
	}
	nonce, err := secureRandomString(32)
	if err != nil {
		return "", "", err
	}
	verifier, err := secureRandomString(48)
	if err != nil {
		return "", "", err
	}
	tx := oidcTransaction{State: state, Nonce: nonce, Verifier: verifier, Expires: time.Now().Add(10 * time.Minute).Unix()}
	cookie, err := s.encodeTransaction(tx)
	if err != nil {
		return "", "", err
	}
	url := s.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	return url, cookie, nil
}

func (s *Service) Complete(ctx context.Context, state, encodedTransaction, code string) (Identity, error) {
	tx, err := s.decodeTransaction(encodedTransaction)
	if err != nil {
		return Identity{}, err
	}
	if time.Now().Unix() > tx.Expires || subtle.ConstantTimeCompare([]byte(state), []byte(tx.State)) != 1 {
		return Identity{}, errors.New("invalid oidc state")
	}
	token, err := s.oauth.Exchange(ctx, code, oauth2.VerifierOption(tx.Verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange oidc code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, errors.New("oidc response has no id_token")
	}
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id_token: %w", err)
	}
	var claims struct {
		Subject           string          `json:"sub"`
		Email             string          `json:"email"`
		Name              string          `json:"name"`
		PreferredUsername string          `json:"preferred_username"`
		Nonce             string          `json:"nonce"`
		Groups            json.RawMessage `json:"groups"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, err
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(tx.Nonce)) != 1 {
		return Identity{}, errors.New("invalid oidc nonce")
	}
	email := strings.TrimSpace(claims.Email)
	if email == "" {
		email = strings.TrimSpace(claims.PreferredUsername)
	}
	if claims.Subject == "" || email == "" {
		return Identity{}, errors.New("oidc identity is missing subject or email")
	}
	groups, err := parseGroups(claims.Groups)
	if err != nil {
		return Identity{}, err
	}
	return Identity{Subject: claims.Subject, Email: email, Name: strings.TrimSpace(claims.Name), Groups: groups}, nil
}

func parseGroups(value json.RawMessage) ([]string, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var groups []string
	if err := json.Unmarshal(value, &groups); err == nil {
		return groups, nil
	}
	var group string
	if err := json.Unmarshal(value, &group); err != nil {
		return nil, errors.New("invalid oidc groups claim")
	}
	if group == "" {
		return nil, nil
	}
	return []string{group}, nil
}

func (s *Service) encodeTransaction(tx oidcTransaction) (string, error) {
	payload, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.signKey)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *Service) decodeTransaction(value string) (oidcTransaction, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return oidcTransaction{}, errors.New("invalid oidc transaction")
	}
	mac, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcTransaction{}, errors.New("invalid oidc transaction")
	}
	expected := hmac.New(sha256.New, s.signKey)
	_, _ = expected.Write([]byte(parts[0]))
	if subtle.ConstantTimeCompare(mac, expected.Sum(nil)) != 1 {
		return oidcTransaction{}, errors.New("invalid oidc transaction")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcTransaction{}, errors.New("invalid oidc transaction")
	}
	var tx oidcTransaction
	if err := json.Unmarshal(payload, &tx); err != nil {
		return oidcTransaction{}, errors.New("invalid oidc transaction")
	}
	return tx, nil
}
