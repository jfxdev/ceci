package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"strings"

	"leaflag/backend/internal/repository"
)

type OIDCConfigurationStatus struct {
	Enabled         bool
	IssuerURL       string
	ClientID        string
	RedirectURL     string
	JITEnabled      bool
	HasClientSecret bool
}
type OIDCConfigurationInput struct {
	Enabled      bool
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	JITEnabled   bool
}
type OIDCConfigurationService struct {
	settings                     repository.InstanceSettingsRepository
	encryptionKey, signingSecret string
}

func NewOIDCConfigurationService(settings repository.InstanceSettingsRepository, encryptionKey, signingSecret string) *OIDCConfigurationService {
	return &OIDCConfigurationService{settings: settings, encryptionKey: encryptionKey, signingSecret: signingSecret}
}
func (s *OIDCConfigurationService) Status(ctx context.Context) (OIDCConfigurationStatus, error) {
	v, err := s.settings.Get(ctx)
	if err != nil {
		return OIDCConfigurationStatus{}, err
	}
	return OIDCConfigurationStatus{Enabled: v.OIDCEnabled, IssuerURL: v.OIDCIssuerURL, ClientID: v.OIDCClientID, RedirectURL: v.OIDCRedirectURL, JITEnabled: v.OIDCJITEnabled, HasClientSecret: v.OIDCSecretCiphertext != ""}, nil
}
func (s *OIDCConfigurationService) Current(ctx context.Context) (*OIDCService, error) {
	v, err := s.settings.Get(ctx)
	if err != nil || !v.OIDCEnabled {
		return nil, err
	}
	secret, err := s.decrypt(v.OIDCSecretCiphertext)
	if err != nil {
		return nil, err
	}
	return NewOIDCService(ctx, OIDCConfig{IssuerURL: v.OIDCIssuerURL, ClientID: v.OIDCClientID, ClientSecret: secret, RedirectURL: v.OIDCRedirectURL, JITEnabled: v.OIDCJITEnabled}, s.signingSecret)
}
func (s *OIDCConfigurationService) Update(ctx context.Context, in OIDCConfigurationInput) (OIDCConfigurationStatus, error) {
	v, err := s.settings.Get(ctx)
	if err != nil {
		return OIDCConfigurationStatus{}, err
	}
	if !in.Enabled {
		v.OIDCEnabled = false
		v.OIDCIssuerURL = ""
		v.OIDCClientID = ""
		v.OIDCSecretCiphertext = ""
		v.OIDCRedirectURL = ""
		v.OIDCJITEnabled = false
		err = s.settings.SetOIDC(ctx, v)
		return OIDCConfigurationStatus{Enabled: false}, err
	}
	secret := strings.TrimSpace(in.ClientSecret)
	if secret == "" {
		if v.OIDCSecretCiphertext == "" {
			return OIDCConfigurationStatus{}, errors.New("client secret is required")
		}
		secret, err = s.decrypt(v.OIDCSecretCiphertext)
		if err != nil {
			return OIDCConfigurationStatus{}, err
		}
	}
	cfg := OIDCConfig{IssuerURL: strings.TrimRight(strings.TrimSpace(in.IssuerURL), "/"), ClientID: strings.TrimSpace(in.ClientID), ClientSecret: secret, RedirectURL: strings.TrimSpace(in.RedirectURL), JITEnabled: in.JITEnabled}
	if _, err = NewOIDCService(ctx, cfg, s.signingSecret); err != nil {
		return OIDCConfigurationStatus{}, err
	}
	encrypted, err := s.encrypt(secret)
	if err != nil {
		return OIDCConfigurationStatus{}, err
	}
	v.OIDCEnabled = true
	v.OIDCIssuerURL = cfg.IssuerURL
	v.OIDCClientID = cfg.ClientID
	v.OIDCSecretCiphertext = encrypted
	v.OIDCRedirectURL = cfg.RedirectURL
	v.OIDCJITEnabled = cfg.JITEnabled
	if err = s.settings.SetOIDC(ctx, v); err != nil {
		return OIDCConfigurationStatus{}, err
	}
	return s.Status(ctx)
}
func (s *OIDCConfigurationService) crypt() (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(s.encryptionKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *OIDCConfigurationService) encrypt(value string) (string, error) {
	a, err := s.crypt()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(append(nonce, a.Seal(nil, nonce, []byte(value), nil)...)), nil
}
func (s *OIDCConfigurationService) decrypt(value string) (string, error) {
	a, err := s.crypt()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(raw) < a.NonceSize() {
		return "", errors.New("invalid oidc secret")
	}
	plain, err := a.Open(nil, raw[:a.NonceSize()], raw[a.NonceSize():], nil)
	return string(plain), err
}
