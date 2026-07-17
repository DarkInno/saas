package identity

import "strings"

type ProviderKind string

const (
	ProviderKindOIDC      ProviderKind = "oidc"
	ProviderKindOAuth2    ProviderKind = "oauth2"
	ProviderKindMagicLink ProviderKind = "magic_link"
	ProviderKindSAML      ProviderKind = "saml"
)

type ProviderKey string

const (
	ProviderGoogle    ProviderKey = "google"
	ProviderGitHub    ProviderKey = "github"
	ProviderMicrosoft ProviderKey = "microsoft"
	ProviderMagicLink ProviderKey = "magic_link"
	ProviderSAML      ProviderKey = "saml"
)

// Provider describes an external identity provider that has already verified
// the login assertion before SaaS receives it.
type Provider struct {
	Key              ProviderKey
	Kind             ProviderKind
	Issuer           string
	AuthorizationURL string
	TokenURL         string
	UserInfoURL      string
	JWKSURL          string
	EntityID         string
	SSOURL           string
	Scopes           []string
	Metadata         map[string]string
}

func (provider Provider) Validate() error {
	if provider.Key == "" || provider.Kind == "" {
		return ErrInvalidIdentity
	}

	switch provider.Kind {
	case ProviderKindOIDC:
		if provider.Issuer == "" {
			return ErrInvalidIdentity
		}
	case ProviderKindOAuth2:
		if provider.AuthorizationURL == "" || provider.TokenURL == "" {
			return ErrInvalidIdentity
		}
	case ProviderKindMagicLink:
		if provider.Key == "" {
			return ErrInvalidIdentity
		}
	case ProviderKindSAML:
		if provider.EntityID == "" || provider.SSOURL == "" {
			return ErrInvalidIdentity
		}
	default:
		return ErrInvalidIdentity
	}
	return nil
}

func GoogleOIDC() Provider {
	return Provider{
		Key:              ProviderGoogle,
		Kind:             ProviderKindOIDC,
		Issuer:           "https://accounts.google.com",
		AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:         "https://oauth2.googleapis.com/token",
		UserInfoURL:      "https://openidconnect.googleapis.com/v1/userinfo",
		JWKSURL:          "https://www.googleapis.com/oauth2/v3/certs",
		Scopes:           []string{"openid", "email", "profile"},
	}
}

func GitHubOAuth() Provider {
	return Provider{
		Key:              ProviderGitHub,
		Kind:             ProviderKindOAuth2,
		AuthorizationURL: "https://github.com/login/oauth/authorize",
		TokenURL:         "https://github.com/login/oauth/access_token",
		UserInfoURL:      "https://api.github.com/user",
		Scopes:           []string{"read:user", "user:email"},
	}
}

func MicrosoftEntraID(tenant string) Provider {
	tenant = strings.TrimSpace(tenant)
	if tenant == "" {
		tenant = "common"
	}
	baseURL := "https://login.microsoftonline.com/" + tenant + "/oauth2/v2.0"

	return Provider{
		Key:              ProviderMicrosoft,
		Kind:             ProviderKindOIDC,
		Issuer:           "https://login.microsoftonline.com/" + tenant + "/v2.0",
		AuthorizationURL: baseURL + "/authorize",
		TokenURL:         baseURL + "/token",
		UserInfoURL:      "https://graph.microsoft.com/oidc/userinfo",
		JWKSURL:          "https://login.microsoftonline.com/" + tenant + "/discovery/v2.0/keys",
		Scopes:           []string{"openid", "email", "profile"},
	}
}

func MagicLink(key ProviderKey) Provider {
	if key == "" {
		key = ProviderMagicLink
	}
	return Provider{
		Key:      key,
		Kind:     ProviderKindMagicLink,
		Metadata: map[string]string{"flow": "magic_link"},
	}
}

func SAML(key ProviderKey, entityID string, ssoURL string) Provider {
	if key == "" {
		key = ProviderSAML
	}
	return Provider{
		Key:      key,
		Kind:     ProviderKindSAML,
		EntityID: entityID,
		SSOURL:   ssoURL,
	}
}

func GenericOIDC(key ProviderKey, issuer string, scopes ...string) Provider {
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}
	return Provider{
		Key:    key,
		Kind:   ProviderKindOIDC,
		Issuer: issuer,
		Scopes: cloneStrings(scopes),
	}
}

func cloneProvider(provider Provider) Provider {
	provider.Scopes = cloneStrings(provider.Scopes)
	provider.Metadata = cloneStringMap(provider.Metadata)
	return provider
}
