// Package oidc bridges a standard OpenID Connect authorization-code flow into
// the post-auth identity mapping package.
//
// It handles provider metadata, authorization URLs, code exchange, ID-token
// verification, nonce checking, and assertion construction. It does not issue
// application sessions or store state; applications must persist and compare
// the returned state, nonce, and PKCE verifier in their own session layer.
package oidc
