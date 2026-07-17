// Package commission defines tenant-scoped commission programs and optional
// in-memory or database/sql persistence. Database connections, migrations,
// payment-provider integration, and payout execution remain host-owned.
// Service keeps tenant drafting, platform approval, host-verified settlement,
// and outbox delivery as separate authorization boundaries. Every Service
// command requires a configured Authorizer; applications must map authenticated
// identities to Actor values before calling Service. Settlement adapters return
// an explicit pending, settled, or rejected provider outcome: only a verified
// terminal result changes a settlement out of submitted.
//
// 包 commission 定义租户范围佣金计划及可选的内存或 database/sql 持久化。
// 数据库连接、迁移、支付服务商集成和打款执行仍由宿主应用负责。Service 将
// 租户草拟、平台审批、宿主验证的结算和 outbox 投递保持为独立的授权边界。
// 所有 Service 命令都必须配置 Authorizer；应用在调用 Service 前应将已认证的
// 身份映射为 Actor。结算适配器必须返回明确的 pending、settled 或 rejected
// 结果；只有经验证的终态结果才会将结算从 submitted 改变。
package commission
