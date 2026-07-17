// Package saas is a production-oriented, ORM-independent toolkit for building
// shared-database, multi-tenant Go products. It makes the required tenant_id
// isolation boundary explicit and provides shared tenant-isolation errors and
// host-boundary checks.
//
// Tenant lifecycle modules live in tenant, plan, subscription, quota, feature,
// and onboarding. The toolkit also includes data adapters, framework middleware,
// identity links, RBAC, audit, and notification capabilities.
//
// 包 saas 是一个面向生产环境、与 ORM 无关的 Go 多租户应用工具包，用于构建共享数据库隔离的 SaaS 产品。
// 它以强制 tenant_id 隔离边界为核心，并提供共享的租户隔离错误和主控端边界检查。
//
// 生命周期模块位于 tenant、plan、subscription、quota、feature 和 onboarding，工具包还提供数据适配器、
// 框架中间件、身份关联、RBAC、审计和通知能力。
package saas
