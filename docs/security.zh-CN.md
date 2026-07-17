# 安全性

[EN](security.md) | [中文](security.zh-CN.md)

## 租户上下文

- 租户操作使用 `core/context.WithTenant`。
- 主机操作使用 `core/context.WithHost`。
- 长生命周期任务应保存租户元数据，并显式重建上下文。
- 租户和主机状态使用私有、包专属的带类型 Context 键，而非字符串或导出的键。`core/context` 外的代码无法构造同一个键，因此 OpenTelemetry、日志和其他库通过标准 `context.WithValue` 写入的值不会与 SaaS 状态碰撞或将其覆盖。
- 请使用 `WithTenant`、`WithHost`、`FromContext` 和 `IsHost`，而不是直接设置 Context 值。后续的 `WithTenant` 或 `WithHost` 调用会有意在其子 Context 中替换 SaaS 状态。

## GORM 防护措施

- 查询、更新、删除、行和计数路径都会添加 `tenant_id = ?`。
- 创建和批量创建会填充租户 ID，并拒绝不匹配的租户值。
- 在租户上下文中，`Unscoped` 会报告错误。
- 在租户上下文中会拒绝原始 SQL。`SafeRaw` 和 `SafeExec` 需要使用 `core/context.WithHost` 创建的上下文。
- 预加载作用域会附加租户过滤。

## Ent 和 sqlx 防护措施

- Ent 集成提供查询过滤器、变更过滤器和变更 hook。
- Ent 创建变更会从上下文设置 `tenant_id`，并拒绝不匹配的租户值。
- Ent 更新和删除变更会获得存储层的租户谓词。
- sqlx API 只重写简单的单表 SELECT/UPDATE/DELETE 语句。连接、排序、限制、returning 子句、注释或多语句等复杂 SQL 会以 `ErrUnsafeSQL` 拒绝。

## 活跃租户强制措施

- HTTP、Gin、Echo、Fiber、Kratos 和 gRPC 租户中间件默认拒绝非活跃租户。
- 也可为在中间件外部创建的可信上下文使用活跃状态防护。

## 身份与 OIDC

- `biz/identity` 只接受已经验证的供应商断言。
- `biz/identity/oidc` 处理标准 OIDC 授权码回调、令牌交换、ID Token 验证、nonce 检查、PKCE verifier 使用、form-post 回调解析和可选的一次性登录状态存储。应用会话、Magic Link 投递、SAML XML 验证、MFA 和 WebAuthn 不属于该包的职责。
- 应用可以直接传入预期的 `State`、`Nonce` 和 `PKCEVerifier`，也可以使用 `LoginStore` / `MemoryLoginStore` 恰好一次地消费待处理登录状态。
- `MemoryLoginStore` 有容量上限并基于 TTL，但它是进程本地的；横向扩展的应用应基于其安全会话或共享缓存层实现 `LoginStore`。
- ID Token 会根据供应商密钥、issuer、audience、过期时间、nonce 和（如存在）`at_hash` 进行验证。
- 供应商端点和重定向 URL 必须使用 HTTPS，除非它们是用于本地开发的 loopback HTTP URL。
- 在接受断言前，必须显式将身份供应商加入允许列表。
- OIDC issuer 验证是严格的；除非应用显式拥有 issuer 策略，否则应避免 Microsoft `common` 等多租户 issuer 快捷方式。
- 默认要求邮箱已验证。只有在上游 IdP 断言以其他方式可信（例如受控的 SAML 连接）时才应禁用它。
- 外部 subject 按租户、供应商和 subject 关联，以防止跨租户身份复用绕过成员资格检查。
- 只有当断言邮箱与存储的用户邮箱相等时才匹配现有用户，并且登录期间不会覆盖现有租户成员角色。
- 生成的用户 ID 是供应商和 subject 的稳定不透明哈希，避免在普通用户 ID 中泄漏原始供应商 subject。

## 缓存隔离

- 租户缓存键使用 `t2:{base64url(tenant_id)}:{key}`。无填充的 Base64URL 租户组成部分让租户/键边界保持明确，即使任一值包含冒号也是如此。
- 版本化 `t2:` 命名空间可避免歧义的旧 `t:` 条目与新租户键重叠。会拒绝已包含任一租户前缀或全局前缀的用户提供键。
- 主机全局键仍为 `g:{key}`，并且需要显式选择启用。
- 内存缓存适配器提供有界构造函数。
- Redis 适配器只存储由缓存层生成的精确键，不使用宽泛的键扫描；请用 `TenantCache` 包装它以实现租户隔离。
- 生产 Redis 客户端应通过 `go-redis` 选项配置适合部署的 TLS、命令超时、重试限制和 OpenTelemetry instrumentation。

## 错误与日志卫生

- `web/gin.ErrorHandler` 返回通用客户端错误。
- Web 适配器返回通用租户错误码。
- gRPC 拦截器返回带有通用消息的 status 错误。
- `obs.Redact` 在日志输出前掩盖常见的敏感字段。
- `obs.RedactSlogAttrs` 会在结构化日志输出前对敏感 `slog` 属性（包括嵌套组）进行脱敏。
- `obs.RecordSpanError` 使用原始错误类型和调用方提供或通用的状态描述记录已清理的 OpenTelemetry 错误事件，避免通过遥测消息意外泄漏租户或机密信息。
- 框架适配器将租户 ID 作为结构化可观测性字段输出，而不是嵌入错误字符串。
- `biz/notification.SESNotifier` 使用官方 AWS SDK v2 `sesv2.SendEmail` 客户端路径，而非手写 SigV4 签名；将显式消息标签映射为 SES 标签；返回安全的投递错误；并将节流/服务器故障视为可重试。
- `biz/notification.ResendNotifier` 使用带 bearer 身份验证、必需 `User-Agent`、可选幂等键、安全状态错误以及针对 `429` 和 `5xx` 响应重试分类的 Resend HTTPS 邮件 API。
- `biz/notification.WebhookNotifier` 除非端点是 loopback 或显式允许不安全 HTTP，否则要求 HTTPS；拒绝 URL userinfo；输出 JSON payload；支持 HMAC 签名；并且不在错误字符串中包含供应商响应体。
