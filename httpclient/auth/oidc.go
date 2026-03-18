package auth

import (
	"net/http"

	"github.com/fabrionai/fabrion-subsystem-client/oidc"
)

// OIDCPasswordAuth implements Authenticator using OIDC resource owner password grant
// with automatic token refresh on 401 responses.
//
// Unlike other Authenticator implementations, this one works at the transport level
// via oidc.AutoRefreshTransport. Use WrapTransport to install it into an HTTP client's
// transport chain.
type OIDCPasswordAuth struct {
	transport *oidc.AutoRefreshTransport
}

// OIDCPasswordConfig holds the configuration for OIDC password grant authentication.
type OIDCPasswordConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
	InitialToken string
}

// NewOIDCPasswordAuth creates a new OIDC password grant authenticator.
// The returned authenticator manages token lifecycle automatically — it obtains
// an initial token (if not provided) and refreshes on 401 responses.
func NewOIDCPasswordAuth(cfg OIDCPasswordConfig, base http.RoundTripper) *OIDCPasswordAuth {
	if base == nil {
		base = http.DefaultTransport
	}
	return &OIDCPasswordAuth{
		transport: oidc.NewAutoRefreshTransport(
			cfg.TokenURL,
			cfg.ClientID,
			cfg.ClientSecret,
			cfg.Username,
			cfg.Password,
			cfg.InitialToken,
			base,
		),
	}
}

// Authenticate is a no-op for OIDCPasswordAuth since auth is handled at the transport level.
// Use WrapTransport instead to install this authenticator.
func (a *OIDCPasswordAuth) Authenticate(_ *http.Request) error {
	return nil
}

// WrapTransport returns the underlying AutoRefreshTransport, which should be used
// as the http.Client's Transport. This transport handles adding the Bearer token
// and refreshing it on 401 responses.
func (a *OIDCPasswordAuth) WrapTransport() http.RoundTripper {
	return a.transport
}
