package oidc

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	identity "github.com/DarkInno/gotenancy/biz/identity"
	"golang.org/x/oauth2"
)

func TestBeginBuildsAuthorizationURLWithStateNonceAndPKCE(t *testing.T) {
	client := testClient(t, testServerOptions{})

	request, err := client.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	parsed, err := url.Parse(request.URL)
	if err != nil {
		t.Fatalf("Parse auth URL error = %v", err)
	}
	values := parsed.Query()
	if values.Get("state") != request.State {
		t.Fatalf("state = %q, want %q", values.Get("state"), request.State)
	}
	if values.Get("nonce") != request.Nonce {
		t.Fatalf("nonce = %q, want %q", values.Get("nonce"), request.Nonce)
	}
	if values.Get("code_challenge") == "" || values.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE query = %q/%q, want S256 challenge", values.Get("code_challenge"), values.Get("code_challenge_method"))
	}
}

func TestCallbackVerifiesTokenAndBuildsAssertion(t *testing.T) {
	ctx := context.Background()
	client := testClient(t, testServerOptions{})
	authRequest, err := client.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	result, err := client.Callback(ctx, Callback{
		Values:        callbackValues(authRequest.Nonce, authRequest.State),
		ExpectedState: authRequest.State,
		ExpectedNonce: authRequest.Nonce,
		PKCEVerifier:  authRequest.PKCEVerifier,
		TenantID:      "tenant-a",
		Roles:         []string{"member"},
	})
	if err != nil {
		t.Fatalf("Callback() error = %v", err)
	}

	assertion := result.Assertion
	if assertion.TenantID != "tenant-a" || assertion.Provider != identity.ProviderGoogle || assertion.Subject != "subject-1" {
		t.Fatalf("Assertion identity = %+v, want tenant/provider/subject", assertion)
	}
	if assertion.Email != "user@example.com" || !assertion.EmailVerified || assertion.Name != "User Example" {
		t.Fatalf("Assertion profile = %+v, want verified profile", assertion)
	}
	if len(assertion.Roles) != 1 || assertion.Roles[0] != "member" {
		t.Fatalf("Assertion roles = %#v, want member", assertion.Roles)
	}
	if result.RawIDToken == "" || result.Token.AccessToken != "access-token" {
		t.Fatalf("Result token = %+v, rawIDToken empty=%v", result.Token, result.RawIDToken == "")
	}
}

func TestCallbackRejectsStateMismatchBeforeExchange(t *testing.T) {
	ctx := context.Background()
	server := newOIDCTestServer(t, testServerOptions{})
	client := newTestClient(t, server)

	_, err := client.Callback(ctx, Callback{
		Values:        callbackValues("code-ok", "bad-state"),
		ExpectedState: "expected-state",
		ExpectedNonce: "expected-nonce",
		PKCEVerifier:  "verifier",
		TenantID:      "tenant-a",
	})
	if err != ErrStateMismatch {
		t.Fatalf("Callback() error = %v, want ErrStateMismatch", err)
	}
	if server.tokenCalls != 0 {
		t.Fatalf("token calls = %d, want 0 before state passes", server.tokenCalls)
	}
}

func TestCallbackRejectsNonceMismatch(t *testing.T) {
	ctx := context.Background()
	client := testClient(t, testServerOptions{})
	authRequest, err := client.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = client.Callback(ctx, Callback{
		Values:        callbackValues(authRequest.Nonce, authRequest.State),
		ExpectedState: authRequest.State,
		ExpectedNonce: "wrong-nonce",
		PKCEVerifier:  authRequest.PKCEVerifier,
		TenantID:      "tenant-a",
	})
	if err != ErrNonceMismatch {
		t.Fatalf("Callback() error = %v, want ErrNonceMismatch", err)
	}
}

func TestCallbackRejectsUserInfoSubjectMismatch(t *testing.T) {
	ctx := context.Background()
	client := testClient(t, testServerOptions{fetchUserInfo: true, userInfoSubject: "other-subject"})
	authRequest, err := client.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = client.Callback(ctx, Callback{
		Values:        callbackValues(authRequest.Nonce, authRequest.State),
		ExpectedState: authRequest.State,
		ExpectedNonce: authRequest.Nonce,
		PKCEVerifier:  authRequest.PKCEVerifier,
		TenantID:      "tenant-a",
	})
	if err != ErrSubjectMismatch {
		t.Fatalf("Callback() error = %v, want ErrSubjectMismatch", err)
	}
}

func TestCallbackRejectsAccessTokenHashMismatch(t *testing.T) {
	ctx := context.Background()
	client := testClient(t, testServerOptions{accessTokenHash: "bad-hash"})
	authRequest, err := client.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_, err = client.Callback(ctx, Callback{
		Values:        callbackValues(authRequest.Nonce, authRequest.State),
		ExpectedState: authRequest.State,
		ExpectedNonce: authRequest.Nonce,
		PKCEVerifier:  authRequest.PKCEVerifier,
		TenantID:      "tenant-a",
	})
	if err == nil {
		t.Fatal("Callback() error = nil, want access token hash verification error")
	}
}

func TestNewRejectsNonOIDCProvider(t *testing.T) {
	_, err := New(context.Background(), Config{
		Provider:    identity.GitHubOAuth(),
		ClientID:    "client-id",
		RedirectURL: "https://app.example.com/callback",
	})
	if err != ErrInvalidConfig {
		t.Fatalf("New(non OIDC) error = %v, want ErrInvalidConfig", err)
	}
}

func testClient(t *testing.T, options testServerOptions) *Client {
	t.Helper()
	server := newOIDCTestServer(t, options)
	return newTestClient(t, server)
}

func newTestClient(t *testing.T, server *oidcTestServer) *Client {
	t.Helper()
	client, err := New(context.Background(), Config{
		Provider: identity.Provider{
			Key:              identity.ProviderGoogle,
			Kind:             identity.ProviderKindOIDC,
			Issuer:           server.issuer,
			AuthorizationURL: server.issuer + "/authorize",
			TokenURL:         server.issuer + "/token",
			UserInfoURL:      server.issuer + "/userinfo",
			JWKSURL:          server.issuer + "/jwks",
			Scopes:           []string{"openid", "email", "profile"},
		},
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		RedirectURL:   "https://app.example.com/callback",
		FetchUserInfo: server.options.fetchUserInfo,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func callbackValues(code string, state string) url.Values {
	return url.Values{"code": {code}, "state": {state}}
}

type testServerOptions struct {
	fetchUserInfo   bool
	userInfoSubject string
	accessTokenHash string
}

type oidcTestServer struct {
	issuer     string
	tokenCalls int
	options    testServerOptions
	key        *rsa.PrivateKey
	server     *httptest.Server
}

func newOIDCTestServer(t *testing.T, options testServerOptions) *oidcTestServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	testServer := &oidcTestServer{key: key, options: options}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", testServer.discovery)
	mux.HandleFunc("/authorize", testServer.notUsed)
	mux.HandleFunc("/token", testServer.token)
	mux.HandleFunc("/jwks", testServer.jwks)
	mux.HandleFunc("/userinfo", testServer.userinfo)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	testServer.server = server
	testServer.issuer = server.URL
	return testServer
}

func (server *oidcTestServer) discovery(writer http.ResponseWriter, _ *http.Request) {
	server.writeJSON(writer, map[string]any{
		"issuer":                                server.issuer,
		"authorization_endpoint":                server.issuer + "/authorize",
		"token_endpoint":                        server.issuer + "/token",
		"jwks_uri":                              server.issuer + "/jwks",
		"userinfo_endpoint":                     server.issuer + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (server *oidcTestServer) notUsed(writer http.ResponseWriter, _ *http.Request) {
	http.Error(writer, "not used", http.StatusBadRequest)
}

func (server *oidcTestServer) token(writer http.ResponseWriter, request *http.Request) {
	server.tokenCalls++
	if err := request.ParseForm(); err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Form.Get("code_verifier") == "" {
		http.Error(writer, "missing verifier", http.StatusBadRequest)
		return
	}

	claims := map[string]any{
		"iss":            server.issuer,
		"sub":            "subject-1",
		"aud":            "client-id",
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Add(-time.Minute).Unix(),
		"nonce":          request.Form.Get("code"),
		"email":          "user@example.com",
		"email_verified": true,
		"name":           "User Example",
	}
	if server.options.accessTokenHash != "" {
		claims["at_hash"] = server.options.accessTokenHash
	}

	token, err := server.signIDToken(claims)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	server.writeJSON(writer, map[string]any{
		"access_token": "access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     token,
	})
}

func (server *oidcTestServer) jwks(writer http.ResponseWriter, _ *http.Request) {
	server.writeJSON(writer, map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "test-key",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(server.key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(server.key.E)).Bytes()),
			},
		},
	})
}

func (server *oidcTestServer) userinfo(writer http.ResponseWriter, _ *http.Request) {
	subject := server.options.userInfoSubject
	if subject == "" {
		subject = "subject-1"
	}
	server.writeJSON(writer, map[string]any{
		"sub":            subject,
		"email":          "user@example.com",
		"email_verified": true,
		"name":           "User Example",
	})
}

func (server *oidcTestServer) writeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func (server *oidcTestServer) signIDToken(claims map[string]any) (string, error) {
	header := map[string]any{"alg": "RS256", "kid": "test-key", "typ": "JWT"}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsJSON)
	unsigned := encodedHeader + "." + encodedClaims
	digest := sha256.Sum256([]byte(unsigned))

	signature, err := rsa.SignPKCS1v15(rand.Reader, server.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		encodedHeader,
		encodedClaims,
		base64.RawURLEncoding.EncodeToString(signature),
	}, "."), nil
}

func TestAuthURLWithExplicitValues(t *testing.T) {
	client := testClient(t, testServerOptions{})

	authorizationURL, err := client.AuthURL("state", "nonce", oauth2.GenerateVerifier())
	if err != nil {
		t.Fatalf("AuthURL() error = %v", err)
	}
	if !strings.Contains(authorizationURL, "state=state") || !strings.Contains(authorizationURL, "nonce=nonce") {
		t.Fatalf("AuthURL() = %q, want state and nonce", authorizationURL)
	}
}
