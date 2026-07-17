package gormtenant

import "gorm.io/gorm"

const pluginName = "saas"

// Plugin injects tenant isolation callbacks into GORM.
type Plugin struct {
	config Config
}

var _ gorm.Plugin = (*Plugin)(nil)

// New creates a GORM tenant plugin.
func New(config Config) *Plugin {
	return &Plugin{config: config.normalize()}
}

// Name returns the GORM plugin name.
func (plugin *Plugin) Name() string {
	return pluginName
}

// Initialize registers tenant callbacks.
func (plugin *Plugin) Initialize(db *gorm.DB) error {
	if err := db.Callback().Query().Before("gorm:query").Register("saas:query", plugin.addTenantCondition); err != nil {
		return err
	}
	if err := db.Callback().Query().Before("gorm:preload").Register("saas:preload", plugin.guardPreloads); err != nil {
		return err
	}
	if err := db.Callback().Create().Before("gorm:create").Register("saas:create", plugin.fillTenantOnCreate); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").Register("saas:update", plugin.guardUpdate); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").Register("saas:delete", plugin.addTenantCondition); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("gorm:row").Register("saas:row", plugin.addTenantCondition); err != nil {
		return err
	}
	if err := db.Callback().Raw().Before("gorm:raw").Register("saas:raw", plugin.requireHostForRaw); err != nil {
		return err
	}
	return nil
}
