package environment

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
)

var verifyIDTokenWithProvider = func(a *Authenticator, ctx context.Context, clientID, rawIDToken string) (*oidc.IDToken, error) {
	oidcConfig := &oidc.Config{ClientID: clientID}
	return a.Verifier(oidcConfig).Verify(ctx, rawIDToken)
}

var userTokenScopes = []string{
	oidc.ScopeOpenID,
	oidc.ScopeOfflineAccess,
	"https://graph.microsoft.com/Mail.Read",
	"https://graph.microsoft.com/User.ReadBasic.All",
}

type Authenticator struct {
	*oidc.Provider
	oauth2.Config
}

type AzureADClaims struct {
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
}

func NewAuthenticator() (*Authenticator, error) {
	gob.Register(oidc.IDToken{})
	gob.Register(oauth2.Token{})
	gob.Register(AzureADClaims{})
	issuer := AzureADIssuerFromConfig()
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider from issuer %q: %w", issuer, err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     viper.GetString("azureAD.clientID"),
		ClientSecret: viper.GetString("azureAD.clientSecret"),
		RedirectURL:  viper.GetString("azureAD.redirectURL"),
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &Authenticator{
		Provider: provider,
		Config:   oauth2Config,
	}, nil
}

func (a *Authenticator) VerifyIDToken(ctx context.Context, token *oauth2.Token) (*oidc.IDToken, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("no id_token field in oauth2 token")
	}

	return verifyIDTokenWithProvider(a, ctx, a.ClientID, rawIDToken)
}

func ParseClaims(token *oidc.IDToken, claims *AzureADClaims) error {
	return token.Claims(claims)
}

// NewUserTokenAuthenticator creates an authenticator for user token authorization
// with mail access scopes including offline_access for refresh tokens
func NewUserTokenAuthenticator() (*Authenticator, error) {
	gob.Register(oauth2.Token{})
	issuer := AzureADIssuerFromConfig()
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider from issuer %q: %w", issuer, err)
	}

	oauth2Config := oauth2.Config{
		ClientID:     viper.GetString("azureAD.clientID"),
		ClientSecret: viper.GetString("azureAD.clientSecret"),
		RedirectURL:  DeriveUserTokenRedirectURL(viper.GetString("azureAD.redirectURL")), // Different callback URL
		Endpoint:     provider.Endpoint(),
		Scopes:       append([]string(nil), userTokenScopes...),
	}

	return &Authenticator{
		Provider: provider,
		Config:   oauth2Config,
	}, nil
}

func AzureADIssuerFromConfig() string {
	issuer := viper.GetString("azureAD.issuer")
	if issuer != "" {
		return issuer
	}
	return azureADEndpoint(viper.GetString("azureAD.tenant"))
}

func DeriveUserTokenRedirectURL(baseRedirectURL string) string {
	u, err := url.Parse(baseRedirectURL)
	if err != nil || u.Path == "" {
		return baseRedirectURL + "-user-token"
	}
	u.Path = u.Path + "-user-token"
	return u.String()
}

func azureADEndpoint(tenant string) string {
	if tenant == "" {
		tenant = "common"
	}

	u := url.URL{
		Scheme: "https",
		Host:   "login.microsoftonline.com",
	}
	return u.JoinPath(tenant, "v2.0").String()
}
