package httpapi

import "github.com/jrojowski/profit-tracker/backend/internal/returns"

// percent returns the rate as a percentage in decimal text, or nil if the rate is not available.
func percent(r returns.Rate) *string {
	if !r.Available() {
		return nil
	}
	s := r.Percent.String()
	return &s
}

// returnNote returns a sentence for the user that tells why the rate is not available,
// or nil if the rate is available.
func returnNote(r returns.Rate) *string {
	var note string
	switch r.Reason {
	case returns.ReasonNone:
		return nil
	case returns.ReasonNoNetDeposits:
		note = "The Net Deposits are 0 or less."
	case returns.ReasonNoFlows:
		note = "There are no deposits."
	case returns.ReasonShortHistory:
		note = "The first deposit is less than one year old."
	case returns.ReasonHoldingNoValue:
		note = "A Holding has no value."
	case returns.ReasonNoRoot:
		note = "No yearly rate gives the current value."
	case returns.ReasonNoConvergence:
		note = "The Annual Return does not converge."
	case returns.ReasonMissingExchangeRate:
		note = "An Exchange Rate is missing. Refresh the prices."
	case returns.ReasonNoValueHistory:
		note = "The Value History has no day with a value before it."
	}
	return &note
}
