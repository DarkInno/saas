# 租户部署单元

[EN](deployment.md) | [中文](deployment.zh-CN.md)

`deployment` 包提供从租户到由宿主应用管理的逻辑部署单元的控制面映射。它适用于需要将租户放置显式化的 SaaS 应用，例如按区域运营、履行合同约定，或落实 GDPR、中国数据本地化等数据驻留策略。

部署单元是元数据，不是基础设施。实际运行拓扑仍由宿主应用拥有并决定：解析到部署单元后如何选择连接、路由请求、放置 Worker、备份和复制数据，均由宿主负责。

## 模型与策略边界

`types.DeploymentUnit` 包含稳定的 `DeploymentUnitID`、启用或停用状态、区域标签、驻留标签以及宿主自定义元数据。例如，宿主可以为单元标注 `eu`、`gdpr`、`cn` 或 `mainland-china`。标签名称及其法律含义完全由宿主定义。

`deployment.Policy` 是由宿主提供的部署决策扩展点：

```go
type Policy interface {
	Validate(context.Context, types.Tenant, types.DeploymentUnit) error
}
```

配置后，宿主通过该策略实施自己的规则，例如只允许欧盟租户使用带有已批准驻留标签的单元。SaaS 不内置司法辖区规则、不对数据分类，也不声明某个标签本身即可满足法律义务。

本包**不会**保存 DSN、凭据、云厂商端点、代理配置、数据库连接池或复制配置；也不会运行代理、执行数据库迁移或搬迁数据。这些属于应用和基础设施职责，位于本库的信任边界之外。

## 分配与请求解析

部署服务维护三类相互关联的控制面记录：

- 部署单元目录；
- 每个已放置租户唯一的一条当前 `Assignment`；
- 每个租户至多一条已准备但尚未切换的 `Move`。

使用 `Assign` 创建租户到单元的初始映射。初始分配为仅创建操作，调用方不能静默覆盖当前单元。配置后，`Policy` 会在初始分配、准备迁移和切换迁移时校验。租户查询成功后，使用 `Resolve` 取得该租户当前的启用单元；映射缺失或单元停用都会阻止启用了部署感知的请求继续处理。

Web 适配器（`web/http`、Gin、Echo、Fiber、Kratos）和 gRPC 拦截器都支持可选的 `WithDeploymentResolver` 配置。配置后，它们先解析租户，再解析其部署单元；成功时通过 `tenantctx.WithTenantDeployment` 同时写入上下文，应用代码可通过 `tenantctx.DeploymentFromContext` 读取该单元。

未配置此选项时，现有租户中间件行为保持不变。配置后，解析失败必须返回通用的“部署不可用”拒绝结果，不能向调用方暴露区域、驻留标签、策略判断或基础设施细节。

部署上下文与租户上下文共同限定作用域。`WithHost`、`Detach` 和 `Switch` 都会清除它，避免宿主侧工作或切换租户后的工作意外继承此前租户的部署位置。

## 受控迁移

变更部署位置必须遵循受控、由宿主驱动的流程，不能直接覆盖当前映射：

1. `PrepareMove` 在校验目标单元后记录源和目标单元；此时 `Resolve` 仍返回源单元。
2. 宿主在本库之外完成数据复制、完整性校验、自有路由或连接配置变更及所需的运维审批。
3. `CutoverMove` 在同一 Store 事务中原子地将当前 `Assignment` 更新到目标，并删除完全匹配的已准备迁移；后续解析返回目标单元。
4. `CancelMove` 放弃已准备的迁移，但保留源单元映射。

服务会拒绝并发或过期迁移、无效或已停用的目标单元，以及仍被当前映射或已准备迁移引用的单元删除/停用。迁移因此是部署位置状态机，而不是数据复制、复制同步、故障切换或回滚机制。

## SQLStore 表结构

`deployment.SQLStore` 使用宿主提供的 `database/sql` 连接，不会创建或迁移表。除非通过表名选项覆盖，否则它要求以下精确默认值：

| 默认表名 | 必需列 | 必需键；建议的查询索引 |
|---|---|---|
| `saas_deployment_units` | `id`、`status`、`region`、`residency_tags`、`metadata` | `id` 主键。 |
| `saas_tenant_deployments` | `tenant_id`、`deployment_unit_id`、`version` | `tenant_id` 主键或唯一键；建议为按部署位置列举建立 `deployment_unit_id` 索引。 |
| `saas_deployment_moves` | `tenant_id`、`source_unit_id`、`target_unit_id` | `tenant_id` 主键或唯一键；建议建立 `source_unit_id` 和 `target_unit_id` 索引用于“正在使用”检查。 |

`residency_tags` 和 `metadata` 必须保存与 Store JSON 编码兼容的 JSON 值（例如 JSON/JSONB 或合法 JSON 文本）。SQL Store 在 Assignment 的比较并交换（CAS）更新中使用 `version`，并在同一个可串行化事务中完成有条件的 Assignment 更新及完全匹配的 Move 删除。它还会在绑定、迁入、停用或删除单元前锁定该单元，因此这些生命周期变更不会在多个服务实例之间互相竞争。

迁移和数据库专属约束仍由宿主决定。若数据库支持，请为 `deployment_unit_id`、`source_unit_id` 和 `target_unit_id` 添加指向 `saas_deployment_units(id)` 的外键；这可为绕过本包的直接数据库访问提供额外完整性保护。

## 审计与可观测性

服务可通过 `deployment.Auditor` 发送已提交的控制面变更：初始分配、迁移准备、切换和取消事件均包含租户及相关源/目标单元 ID。审计接收端失败必须暴露出来以便补救，但不能追溯撤销已经提交的部署位置变更。

`obs` 和 OpenTelemetry 辅助工具仅在现有租户字段旁增加 `deployment_unit_id`。它们刻意不输出区域标签、驻留标签或任意宿主元数据。需要更丰富审计记录的宿主应写入受访问控制的审计系统，而不是通用请求遥测。

## 既有数据隔离模型

部署映射不会改变 SaaS 目前支持的数据模型：本库仍采用共享应用数据库和共享 Schema，每一行租户数据都必须带有 `tenant_id`。它不实现按租户独立数据库、独立 Schema、混合隔离或数据库路由。

宿主可以利用解析出的部署单元选择区域化数据库或服务拓扑，但该路由及保障其安全所需的数据迁移仍完全由宿主拥有。租户作用域数据适配器仍从 `context.Context` 和 `tenant_id` 推导隔离边界。
