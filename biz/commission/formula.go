package commission

import (
	"math"
	"sort"
)

const basisPointDenominator int64 = 10_000

// Calculate derives commission calculations for the effective beneficiary
// rules in template. Tier ranges are half-open [MinMinor, MaxMinor), except
// that MaxMinor == 0 is unbounded. Overlapping tiers intentionally accumulate.
func Calculate(template Template, event CommissionEvent) ([]Calculation, error) {
	if err := validateCommissionEvent(event); err != nil {
		return nil, err
	}
	if err := validateCalculationTemplate(template); err != nil {
		return nil, err
	}
	if !allowsEventType(template.AllowedEventTypes, event.SourceType) {
		return nil, ErrEventTypeDisabled
	}

	capEnabled := template.MaxCommission.Minor > 0
	if capEnabled && template.MaxCommission.Currency != event.Commissionable.Currency {
		return nil, ErrCurrencyMismatch
	}

	calculations := make([]Calculation, 0, len(template.Rules))
	var total int64
	for _, rule := range template.Rules {
		if err := validateRule(rule, true); err != nil {
			return nil, err
		}
		minor, err := calculateRule(rule, event.Commissionable.Minor)
		if err != nil {
			return nil, err
		}
		if minor == 0 {
			continue
		}
		if total > math.MaxInt64-minor {
			return nil, ErrAmountOverflow
		}
		total += minor
		if capEnabled && total > template.MaxCommission.Minor {
			return nil, ErrCommissionCapExceeded
		}
		calculations = append(calculations, Calculation{
			Slot:        rule.Slot,
			Beneficiary: rule.Beneficiary,
			Amount: Amount{
				Currency: event.Commissionable.Currency,
				Minor:    minor,
			},
		})
	}
	return calculations, nil
}

func validateCalculationTemplate(template Template) error {
	if template.FreezePeriod < 0 || !optionalAmountValid(template.MaxCommission) {
		return ErrInvalidTemplate
	}
	for _, eventType := range template.AllowedEventTypes {
		if eventType == "" {
			return ErrInvalidTemplate
		}
	}
	for _, rule := range template.Rules {
		if err := validateRule(rule, false); err != nil {
			return err
		}
	}
	return nil
}

func allowsEventType(allowed []string, sourceType string) bool {
	for _, eventType := range allowed {
		if eventType == sourceType {
			return true
		}
	}
	return false
}

func calculateRule(rule Rule, commissionableMinor int64) (int64, error) {
	tiers := append([]Tier(nil), rule.Tiers...)
	sort.SliceStable(tiers, func(i, j int) bool {
		return tiers[i].MinMinor < tiers[j].MinMinor
	})

	var total int64
	for _, tier := range tiers {
		intersection := tierIntersection(tier, commissionableMinor)
		percentage, err := percentageMinor(intersection, tier.BasisPoints)
		if err != nil {
			return 0, err
		}
		if total > math.MaxInt64-percentage {
			return 0, ErrAmountOverflow
		}
		total += percentage

		if tierContains(tier, commissionableMinor) {
			if total > math.MaxInt64-tier.FixedMinor {
				return 0, ErrAmountOverflow
			}
			total += tier.FixedMinor
		}
	}
	return total, nil
}

func tierIntersection(tier Tier, amount int64) int64 {
	if amount <= tier.MinMinor {
		return 0
	}
	upper := amount
	if tier.MaxMinor != 0 && upper > tier.MaxMinor {
		upper = tier.MaxMinor
	}
	if upper <= tier.MinMinor {
		return 0
	}
	return upper - tier.MinMinor
}

func tierContains(tier Tier, amount int64) bool {
	return amount >= tier.MinMinor && (tier.MaxMinor == 0 || amount < tier.MaxMinor)
}

func percentageMinor(minor int64, basisPoints int64) (int64, error) {
	if minor < 0 || basisPoints < 0 || basisPoints > basisPointDenominator {
		return 0, ErrInvalidTier
	}
	whole := minor / basisPointDenominator
	remainder := minor % basisPointDenominator
	return whole*basisPoints + remainder*basisPoints/basisPointDenominator, nil
}
