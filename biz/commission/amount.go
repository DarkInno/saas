package commission

const currencyCodeLength = 3

// Amount represents a non-negative monetary value in minor units. Currency is
// an uppercase three-letter ISO-4217-style code, such as USD or CNY.
type Amount struct {
	Currency string
	Minor    int64
}

// Validate verifies the amount's currency-code form and non-negative minor value.
func (amount Amount) Validate() error {
	if !validAmount(amount) {
		return ErrInvalidAmount
	}
	return nil
}

func validAmount(amount Amount) bool {
	if amount.Minor < 0 || len(amount.Currency) != currencyCodeLength {
		return false
	}
	for i := 0; i < len(amount.Currency); i++ {
		if amount.Currency[i] < 'A' || amount.Currency[i] > 'Z' {
			return false
		}
	}
	return true
}

func optionalAmountValid(amount Amount) bool {
	return amount == (Amount{}) || validAmount(amount)
}
