package gormtenant

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Scopes creates reusable GORM scopes for tenant-aware operations.
type Scopes struct {
	config Config
}

// NewScopes creates tenant-aware GORM scopes.
func NewScopes(config Config) Scopes {
	return Scopes{config: config.normalize()}
}

// TenantScope adds the current tenant filter to a GORM query. Host context is a no-op.
func (scopes Scopes) TenantScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	plugin := New(scopes.config)
	return func(db *gorm.DB) *gorm.DB {
		filter, err := plugin.newFilter(ctx)
		if err != nil {
			addDBError(db, err)
			return db
		}
		if filter.IsHost() {
			return db
		}
		condition := filter.Condition()
		if condition.Empty() {
			return db
		}
		return db.Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: condition.Expression, Vars: condition.Args},
		}})
	}
}

// TenantScope adds the current tenant filter with default configuration.
func TenantScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return NewScopes(Config{}).TenantScope(ctx)
}
