# Tenant Deployment Units

[EN](deployment.md) | [中文](deployment.zh-CN.md)

The `deployment` package adds a control-plane mapping from a tenant to a
host-managed logical deployment unit. It is intended for SaaS applications
that must make tenant placement explicit for regional operations, contractual
commitments, or data-residency policies such as GDPR and China data-locality
requirements.

A deployment unit is metadata, not infrastructure. The host application owns
the actual runtime topology and decides how a resolved unit affects connection
selection, request routing, worker placement, backup, and data replication.

## Model and policy boundary

`types.DeploymentUnit` contains a stable `DeploymentUnitID`, an active or
disabled status, a region label, residency tags, and host-defined metadata.
For example, a host may label a unit with `eu`, `gdpr`, `cn`, or
`mainland-china` tags. Tag names and their legal meaning are entirely
host-defined.

`deployment.Policy` is the host-provided extension point for placement decisions:

```go
type Policy interface {
	Validate(context.Context, types.Tenant, types.DeploymentUnit) error
}
```

When configured, the host supplies this policy to enforce its own rules, such
as allowing an EU tenant only in units carrying its approved residency tags.
SaaS does not encode jurisdictional rules, classify data, or assert that any
tag by itself satisfies a legal obligation.

The package does **not** store DSNs, credentials, provider endpoints, proxy
configuration, database pools, or replication settings. It also does not run
a proxy or perform database/data migration. Those are application and
infrastructure responsibilities outside this library's trust boundary.

## Assignment and request resolution

The deployment service persists three related control-plane records:

- a catalog of deployment units;
- one current `Assignment` for each placed tenant; and
- at most one prepared `Move` for each tenant.

Use `Assign` to create the initial tenant-to-unit mapping. An initial
assignment is create-only: callers cannot silently overwrite the current unit.
When configured, `Policy` validation is applied when assigning, preparing a
move, and cutting a move over. Use `Resolve` after tenant lookup to obtain the
tenant's current active unit; a missing assignment or disabled unit prevents
deployment-aware request processing.

The web adapters (`web/http`, Gin, Echo, Fiber, and Kratos) and the gRPC
interceptors accept an optional `WithDeploymentResolver` configuration. When
configured, they resolve the tenant first and then resolve its deployment unit.
On success they attach both values with
`tenantctx.WithTenantDeployment`; application code can read the unit through
`tenantctx.DeploymentFromContext`.

When this option is not configured, existing tenant middleware behavior is
unchanged. When it is configured, resolution failures return a generic
deployment-unavailable denial and must not reveal a region, residency tag,
policy decision, or infrastructure detail to the caller.

Deployment context is deliberately scoped with tenant context. `WithHost`,
`Detach`, and `Switch` clear it, so host-side and switched-tenant work cannot
accidentally inherit a previous tenant's placement.

## Controlled moves

Changing placement is a controlled, host-operated sequence rather than a
direct assignment overwrite:

1. `PrepareMove` records a source and target unit after validating the target.
   `Resolve` continues returning the source unit.
2. The host copies data, checks integrity, changes its own routing or
   connection configuration, and completes any operational approval outside
   this library.
3. `CutoverMove` atomically updates the current assignment to the target and
   removes that exact prepared move in one store transaction. Later resolution
   returns the target.
4. `CancelMove` abandons the prepared move while leaving the source assignment
   intact.

The service rejects concurrent or stale moves, invalid/disabled targets, and
unit deletion or disabling while that unit is still referenced by an
assignment or prepared move. A move is therefore a placement state machine;
it is not a data-copy, replication, failover, or rollback mechanism.

## SQLStore schema

`deployment.SQLStore` uses a host-supplied `database/sql` connection and does
not create or migrate tables. Unless table-name options override them, it
expects these exact defaults:

| Default table | Required columns | Required key; recommended lookup indexes |
|---|---|---|
| `saas_deployment_units` | `id`, `status`, `region`, `residency_tags`, `metadata` | Primary key on `id`. |
| `saas_tenant_deployments` | `tenant_id`, `deployment_unit_id`, `version` | Primary or unique key on `tenant_id`; index `deployment_unit_id` for placement listings. |
| `saas_deployment_moves` | `tenant_id`, `source_unit_id`, `target_unit_id` | Primary or unique key on `tenant_id`; indexes on `source_unit_id` and `target_unit_id` for in-use checks. |

`residency_tags` and `metadata` must store JSON values compatible with the
store's JSON encoding (for example JSON/JSONB or valid JSON text). The SQL
store uses `version` in its assignment compare-and-swap update and performs a
cutover's conditional assignment update plus exact move removal in one
serializable transaction. It also locks a unit before binding, moving to,
disabling, or deleting it, so those lifecycle transitions cannot race each
other across service instances.

Hosts choose the migration and database-specific constraints. Add foreign keys
from `deployment_unit_id`, `source_unit_id`, and `target_unit_id` to
`saas_deployment_units(id)` where the database supports them; they provide a
useful integrity backstop for direct database access outside this package.

## Auditing and observability

The service can send committed control-plane changes to a
`deployment.Auditor`: assignment, move preparation, cutover, and cancellation
events include the tenant and relevant source/target unit IDs. An audit sink
failure must be surfaced for remediation, but it does not retroactively undo a
committed placement change.

`obs` and OpenTelemetry helpers expose only `deployment_unit_id` alongside the
existing tenant fields. They intentionally do not emit region labels,
residency tags, or arbitrary host metadata. Hosts that require richer audit
records should emit them through an access-controlled audit system rather than
general request telemetry.

## Existing data-isolation model

Deployment mapping does not change SaaS's supported data model: the library
continues to support a shared application database and shared schema, with a
required `tenant_id` on every tenant-owned row. It does not implement
database-per-tenant, schema-per-tenant, hybrid isolation, or database routing.

A host may use a resolved deployment unit to select a region-specific database
or service topology, but that routing and the data movement needed to make it
safe remain host-owned. Tenant-scoped data adapters continue deriving their
isolation boundary from `context.Context` and `tenant_id`.
