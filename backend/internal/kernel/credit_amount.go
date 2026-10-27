package kernel

import "errors"

const CreditQuantumMicrocredits int64 = 10_000

// RoundCreditChargeUp applies the product's 0.01-credit charge granularity.
// Provider unit prices and cost accounting retain their original precision.
func RoundCreditChargeUp(amount int64) (int64, error) {
	if amount < 0 {
		return 0, errors.New("积分金额无效")
	}
	remainder := amount % CreditQuantumMicrocredits
	if remainder == 0 {
		return amount, nil
	}
	increment := CreditQuantumMicrocredits - remainder
	if amount > (1<<63-1)-increment {
		return 0, errors.New("积分金额无效")
	}
	return amount + increment, nil
}
