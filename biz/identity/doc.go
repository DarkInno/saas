// Package identity maps verified external auth and SSO identities to
// SaaS users and tenant memberships.
//
// The package intentionally does not implement password authentication,
// OAuth callback handlers, ID-token validation, magic-link delivery, or SAML
// XML validation. Applications should verify provider assertions with their
// IdP SDK or protocol library first, then pass the verified identity into this
// package to create a tenant-scoped user/member link.
//
// 包 identity 将经过验证的外部认证和 SSO 身份映射到 SaaS 用户及租户成员关系。
//
// 该包有意不实现密码认证、OAuth 回调处理程序、ID 令牌验证、魔术链接投递或 SAML
// XML 验证。应用应先通过其 IdP SDK 或协议库验证提供方断言，随后将已验证身份传给本包，
// 以创建限定在租户范围内的用户/成员关联。
package identity
