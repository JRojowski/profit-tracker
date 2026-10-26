// Package returns calculates the Simple Return and the Annual Return. See GLOSSARY.md.
// It is a pure calculation with no I/O.
package returns

import (
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

// Reason tells why the tool cannot calculate a return.
type Reason int

const (
	// ReasonNone tells that the return is available.
	ReasonNone Reason = iota
	// ReasonNoNetDeposits tells that the Net Deposits are 0 or less, so the Simple Return has no meaning.
	ReasonNoNetDeposits
	// ReasonNoFlows tells that there is no Net Deposits flow.
	ReasonNoFlows
	// ReasonShortHistory tells that the first Net Deposits flow is less than 365 days before today.
	// A short history gives an extreme yearly rate.
	ReasonShortHistory
	// ReasonHoldingNoValue tells that a Holding has no value. The value of the Account then leaves the Holding out,
	// so a rate from it is wrong. The caller that knows the Holdings sets this reason.
	ReasonHoldingNoValue
	// ReasonNoRoot tells that no yearly rate makes the flows grow to the value, for example when the value is
	// 0 or less.
	ReasonNoRoot
	// ReasonNoConvergence tells that the calculation of the Annual Return does not converge: the solver does not
	// get to the tolerance, or it cannot calculate a power.
	ReasonNoConvergence
	// ReasonMissingExchangeRate tells that the Exchange Rate of the day of a flow is missing, so the flows in PLN
	// are not known. The caller that converts the flows sets this reason. ADR 0004, rule 7.
	ReasonMissingExchangeRate
)

// Rate is a rate of return as a percentage, or the reason why the tool cannot calculate it.
type Rate struct {
	// Percent is the rate as a percentage: 10 is 10 %. It is zero if the rate is not available.
	Percent decimal.Decimal
	// Reason is ReasonNone if the rate is available.
	Reason Reason
}

// Available tells if the rate is available.
func (r Rate) Available() bool {
	return r.Reason == ReasonNone
}

var hundred = decimal.NewFromInt(100)

// Simple returns the Simple Return: totalResult × 100 / netDeposits. The multiplication comes first,
// so an exact result stays exact. The division keeps decimal.DivisionPrecision (16) decimal places.
// The Simple Return is not available if netDeposits is 0 or less.
func Simple(totalResult, netDeposits decimal.Decimal) Rate {
	if !netDeposits.IsPositive() {
		return Rate{Reason: ReasonNoNetDeposits}
	}
	return Rate{Percent: totalResult.Mul(hundred).Div(netDeposits)}
}

// Flow is one Net Deposits flow.
type Flow struct {
	Time time.Time
	// Amount is positive for money that the user puts into the Account: a deposit or a Transfer In.
	// It is negative for money that the user takes from the Account: a withdrawal or a Transfer Out.
	Amount decimal.Decimal
}

// minHistoryDays is the smallest number of days from the first flow to today that gives an Annual Return.
const minHistoryDays = 365

// Annual returns the Annual Return: the yearly rate r that makes the flows grow to value today.
// value is the current value. It counts as money that the user takes from the Account today. Thus r is the root of
//
//	Σ Amount_i × (1 + r)^(days_i / 365) − value = 0
//
// where days_i is the number of days from the day of flow i to today. This is the net present value
// of the flows at r, multiplied by (1 + r)^(days of the first flow / 365). Both have the same root.
//
// Annual counts days between calendar days in the location of today, so the time of day has no effect.
// The result keeps 8 decimal places of the percentage.
//
// allHoldingsHaveValue tells that the current value includes each Holding. It is false when the tool cannot
// calculate the amount of a Holding, for example because the Holding has no price.
//
// The Annual Return is not available in these cases. Annual checks them in this order:
//   - There is no flow.
//   - The first flow is less than 365 days before today.
//   - The current value does not include each Holding. A short history comes first,
//     because a price refresh does not change it.
//   - No rate in the bracket of the solver is a root.
//   - The calculation does not converge.
func Annual(flows []Flow, value decimal.Decimal, allHoldingsHaveValue bool, today time.Time) Rate {
	if reason := history(flows, today); reason != ReasonNone {
		return Rate{Reason: reason}
	}
	if !allHoldingsHaveValue {
		return Rate{Reason: ReasonHoldingNoValue}
	}

	terms := make([]term, 0, len(flows))
	year := decimal.NewFromInt(365)
	for _, f := range flows {
		if f.Amount.IsZero() {
			continue
		}
		terms = append(terms, term{amount: f.Amount, years: decimal.NewFromInt(days(f.Time, today)).Div(year)})
	}
	growth, reason := solve(func(x decimal.Decimal) (fx, dfx decimal.Decimal, ok bool) { return futureValue(terms, value, x) })
	if reason != ReasonNone {
		return Rate{Reason: reason}
	}
	return Rate{Percent: growth.Sub(decimal.NewFromInt(1)).Mul(hundred).Round(8)}
}

// history returns ReasonNoFlows if there is no flow, ReasonShortHistory if the first flow is less than
// 365 days before today, and ReasonNone otherwise.
func history(flows []Flow, today time.Time) Reason {
	if len(flows) == 0 {
		return ReasonNoFlows
	}
	first := slices.MinFunc(flows, func(a, b Flow) int { return a.Time.Compare(b.Time) })
	if days(first.Time, today) < minHistoryDays {
		return ReasonShortHistory
	}
	return ReasonNone
}

// days returns the number of calendar days from the day of from to the day of to, in the location of to.
func days(from, to time.Time) int64 {
	day := func(t time.Time) time.Time {
		y, m, d := t.In(to.Location()).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	return int64(day(to).Sub(day(from)) / (24 * time.Hour))
}

// term is one flow in the equation of Annual.
type term struct {
	amount decimal.Decimal
	// years is the time from the flow to today: days / 365.
	years decimal.Decimal
}

// powPrecision is the number of decimal places of each power in futureValue.
const powPrecision = 24

// futureValue returns f(x) = Σ amount × x^years − value and its derivative f'(x) = Σ years × amount × x^years / x,
// where x is 1 + r. It returns false if a power cannot be calculated.
func futureValue(terms []term, value, x decimal.Decimal) (fx, dfx decimal.Decimal, ok bool) {
	fx, dfx = value.Neg(), decimal.Zero
	for _, t := range terms {
		grown, err := x.PowWithPrecision(t.years, powPrecision)
		if err != nil {
			return decimal.Zero, decimal.Zero, false
		}
		grown = grown.Mul(t.amount)
		fx = fx.Add(grown)
		dfx = dfx.Add(grown.Mul(t.years))
	}
	return fx, dfx.DivRound(x, powPrecision), true
}

// The solver looks for x = 1 + r in the bracket from xLow to xHigh: a rate from −99.9999 % to 999900 %.
var (
	xLow      = decimal.New(1, -6)
	xHigh     = decimal.New(1, 4)
	xFirst    = decimal.NewFromInt(1)
	tolerance = decimal.New(1, -12)
	two       = decimal.NewFromInt(2)
)

// maxIterations is the number of steps after which the solver stops with ReasonNoConvergence.
const maxIterations = 200

// solve finds a root of f in the bracket from xLow to xHigh. f returns the value and the derivative at x.
//
// The solver is Newton's method, made safe with bisection ("rtsafe" in Numerical Recipes, section 9.4).
// Newton's method converges fast near the root. But it can jump out of the domain x > 0, or it can cycle.
// Thus the solver keeps a bracket with a sign change of f. A bracket always has a root.
// The solver uses a bisection step when the Newton step leaves the bracket.
// It also uses a bisection step when the Newton step does not make the bracket smaller fast enough.
//
// It returns ReasonNoRoot if f has the same sign at the two ends of the bracket.
// It returns ReasonNoConvergence if f fails, or if the solver does not get to the tolerance in maxIterations steps.
// With withdrawals, f can have more than one root. Then the solver returns one of them.
func solve(f func(x decimal.Decimal) (fx, dfx decimal.Decimal, ok bool)) (decimal.Decimal, Reason) {
	fLow, _, okLow := f(xLow)
	fHigh, _, okHigh := f(xHigh)
	switch {
	case !okLow || !okHigh:
		return decimal.Zero, ReasonNoConvergence
	case fLow.IsZero():
		return xLow, ReasonNone
	case fHigh.IsZero():
		return xHigh, ReasonNone
	case fLow.Sign() == fHigh.Sign():
		return decimal.Zero, ReasonNoRoot
	}
	// neg is the end of the bracket where f is negative, pos is the end where f is positive.
	neg, pos := xLow, xHigh
	if fLow.IsPositive() {
		neg, pos = xHigh, xLow
	}

	x := xFirst
	// last is the size of the last step. beforeLast is the size of the step before it.
	last, beforeLast := xHigh.Sub(xLow), xHigh.Sub(xLow)
	for range maxIterations {
		fx, dfx, ok := f(x)
		switch {
		case !ok:
			return decimal.Zero, ReasonNoConvergence
		case fx.IsZero():
			return x, ReasonNone
		case fx.IsNegative():
			neg = x
		default:
			pos = x
		}

		next, newton := x, !dfx.IsZero()
		if newton {
			next = x.Sub(fx.DivRound(dfx, powPrecision))
			// Bisect when the Newton step leaves the bracket, or when it is not smaller than half of the step
			// before the last one: |f / f'| > |beforeLast| / 2.
			newton = inside(next, neg, pos) && fx.Mul(two).Abs().LessThanOrEqual(beforeLast.Mul(dfx).Abs())
		}
		if !newton {
			next = neg.Add(pos).Div(two)
		}
		next = next.Round(powPrecision)
		beforeLast, last = last, next.Sub(x).Abs()
		x = next
		if last.LessThan(tolerance) {
			return x, ReasonNone
		}
	}
	return decimal.Zero, ReasonNoConvergence
}

// inside tells if x is strictly between a and b.
func inside(x, a, b decimal.Decimal) bool {
	lo, hi := decimal.Min(a, b), decimal.Max(a, b)
	return x.GreaterThan(lo) && x.LessThan(hi)
}
