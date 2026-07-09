package gormtenant

import "gorm.io/gorm"

func (plugin *Plugin) guardPreloads(tx *gorm.DB) {
	if len(tx.Statement.Preloads) == 0 {
		return
	}

	filter, err := plugin.newFilter(tx.Statement.Context)
	if err != nil {
		addDBError(tx, err)
		return
	}
	if filter.IsHost() {
		return
	}
	if tx.Statement.Unscoped {
		addDBError(tx, ErrUnscopedRequiresHost)
		return
	}

	scope := NewScopes(plugin.config).TenantScope(tx.Statement.Context)
	for name, args := range tx.Statement.Preloads {
		if hasTenantScope(args) {
			continue
		}
		tx.Statement.Preloads[name] = append(args, scope)
	}
}

func hasTenantScope(args []interface{}) bool {
	for _, arg := range args {
		if _, ok := arg.(tenantScopeMarker); ok {
			return true
		}
	}
	return false
}

type tenantScopeMarker interface {
	tenantScopeMarker()
}
