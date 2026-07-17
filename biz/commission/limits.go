package commission

import "strings"

// ServiceLimits bounds untrusted Service inputs and list pages. Zero-valued
// fields use DefaultServiceLimits; negative values and an invalid page range
// make the Service fail closed with ErrLimitExceeded.
//
// Store is intentionally a trusted persistence port, so these request bounds
// are enforced at Service rather than silently changing direct Store behavior.
type ServiceLimits struct {
	MaxIdentifierBytes     int
	MaxEventAttributes     int
	MaxEventAttributeBytes int
	MaxEventTypes          int
	MaxRulesPerTemplate    int
	MaxTiersPerRule        int
	MaxSettlementItems     int
	MaxDueBatch            int
	MaxFilterStatuses      int
	DefaultPageSize        int
	MaxPageSize            int
}

// DefaultServiceLimits returns conservative request limits suitable for normal
// API traffic. Hosts may lower or raise them with WithLimits, while retaining
// a finite value for every externally supplied collection.
func DefaultServiceLimits() ServiceLimits {
	return ServiceLimits{
		MaxIdentifierBytes:     256,
		MaxEventAttributes:     64,
		MaxEventAttributeBytes: 8 << 10,
		MaxEventTypes:          64,
		MaxRulesPerTemplate:    64,
		MaxTiersPerRule:        32,
		MaxSettlementItems:     500,
		MaxDueBatch:            500,
		MaxFilterStatuses:      16,
		DefaultPageSize:        100,
		MaxPageSize:            500,
	}
}

func normalizeServiceLimits(limits ServiceLimits) (ServiceLimits, error) {
	defaults := DefaultServiceLimits()
	values := []*int{
		&limits.MaxIdentifierBytes,
		&limits.MaxEventAttributes,
		&limits.MaxEventAttributeBytes,
		&limits.MaxEventTypes,
		&limits.MaxRulesPerTemplate,
		&limits.MaxTiersPerRule,
		&limits.MaxSettlementItems,
		&limits.MaxDueBatch,
		&limits.MaxFilterStatuses,
		&limits.DefaultPageSize,
		&limits.MaxPageSize,
	}
	defaultValues := []int{
		defaults.MaxIdentifierBytes,
		defaults.MaxEventAttributes,
		defaults.MaxEventAttributeBytes,
		defaults.MaxEventTypes,
		defaults.MaxRulesPerTemplate,
		defaults.MaxTiersPerRule,
		defaults.MaxSettlementItems,
		defaults.MaxDueBatch,
		defaults.MaxFilterStatuses,
		defaults.DefaultPageSize,
		defaults.MaxPageSize,
	}
	for index, value := range values {
		if *value < 0 {
			return defaults, ErrLimitExceeded
		}
		if *value == 0 {
			*value = defaultValues[index]
		}
	}
	if limits.DefaultPageSize > limits.MaxPageSize {
		return defaults, ErrLimitExceeded
	}
	return limits, nil
}

func (limits ServiceLimits) identifierOK(value string) bool {
	return value != "" && len(value) <= limits.MaxIdentifierBytes
}

func (limits ServiceLimits) validateEvent(event CommissionEvent) error {
	if !limits.identifierOK(event.SourceType) || !limits.identifierOK(event.SourceID) {
		return ErrLimitExceeded
	}
	if len(event.Attributes) > limits.MaxEventAttributes {
		return ErrLimitExceeded
	}
	bytes := 0
	for key, value := range event.Attributes {
		if strings.TrimSpace(key) == "" || len(key) > limits.MaxIdentifierBytes {
			return ErrLimitExceeded
		}
		if len(value) > limits.MaxEventAttributeBytes || len(key) > limits.MaxEventAttributeBytes-bytes {
			return ErrLimitExceeded
		}
		bytes += len(key)
		if len(value) > limits.MaxEventAttributeBytes-bytes {
			return ErrLimitExceeded
		}
		bytes += len(value)
	}
	return nil
}

func (limits ServiceLimits) validateTemplate(template Template) error {
	if !limits.identifierOK(template.ID) || len(template.AllowedEventTypes) > limits.MaxEventTypes || len(template.Rules) > limits.MaxRulesPerTemplate {
		return ErrLimitExceeded
	}
	for _, eventType := range template.AllowedEventTypes {
		if !limits.identifierOK(eventType) {
			return ErrLimitExceeded
		}
	}
	return limits.validateRules(template.Rules)
}

func (limits ServiceLimits) validateProgram(program Program) error {
	if !limits.identifierOK(program.ID) || !limits.identifierOK(program.TemplateID) || len(program.Rules) > limits.MaxRulesPerTemplate {
		return ErrLimitExceeded
	}
	return limits.validateRules(program.Rules)
}

func (limits ServiceLimits) validateRules(rules []Rule) error {
	for _, rule := range rules {
		if !limits.identifierOK(rule.Slot) || len(rule.Tiers) > limits.MaxTiersPerRule {
			return ErrLimitExceeded
		}
		if rule.Beneficiary.ID != "" && !limits.identifierOK(rule.Beneficiary.ID) {
			return ErrLimitExceeded
		}
	}
	return nil
}

func (limits ServiceLimits) validateAttribution(attribution Attribution) error {
	if !limits.identifierOK(attribution.ProgramID) || !limits.identifierOK(attribution.Slot) || !limits.identifierOK(attribution.Beneficiary.ID) {
		return ErrLimitExceeded
	}
	return nil
}

func (limits ServiceLimits) normalizeEarningFilter(filter EarningFilter) (EarningFilter, error) {
	if !filter.valid() {
		return EarningFilter{}, ErrInvalidEarningFilter
	}
	if len(filter.ProgramID) > limits.MaxIdentifierBytes || len(filter.Cursor) > limits.MaxIdentifierBytes || len(filter.Statuses) > limits.MaxFilterStatuses {
		return EarningFilter{}, ErrLimitExceeded
	}
	if filter.Beneficiary != nil && !limits.identifierOK(filter.Beneficiary.ID) {
		return EarningFilter{}, ErrLimitExceeded
	}
	if filter.Limit == 0 {
		filter.Limit = limits.DefaultPageSize
	} else if filter.Limit > limits.MaxPageSize {
		return EarningFilter{}, ErrLimitExceeded
	}
	return filter, nil
}

func (limits ServiceLimits) normalizeOutboxFilter(filter OutboxFilter) (OutboxFilter, error) {
	if !filter.valid() {
		return OutboxFilter{}, ErrInvalidOutboxFilter
	}
	if len(filter.Cursor.ID) > limits.MaxIdentifierBytes {
		return OutboxFilter{}, ErrLimitExceeded
	}
	if filter.Limit == 0 {
		filter.Limit = limits.DefaultPageSize
	} else if filter.Limit > limits.MaxPageSize {
		return OutboxFilter{}, ErrLimitExceeded
	}
	return filter, nil
}
