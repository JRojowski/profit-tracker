package returns_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

var today = time.Date(2026, 10, 23, 9, 30, 0, 0, time.UTC)

// flow returns a flow of the amount at the days before today.
func flow(amount string, days int) returns.Flow {
	return returns.Flow{Time: today.AddDate(0, 0, -days), Amount: decimal.RequireFromString(amount)}
}

// TestAnnual checks the branches of the solver that a test at the HTTP API seam cannot easily give.
// The HTTP API tests check the example of spec #13 and the other reasons.
// The expected rates come from an independent bisection in floating point, to 6 decimal places of the percentage.
func TestAnnual(t *testing.T) {
	t.Parallel()
	deposits := []returns.Flow{flow("1000", 800), flow("500", 400), flow("-200", 100)}
	tests := []struct {
		name  string
		flows []returns.Flow
		value string
		want  returns.Rate
	}{
		{
			name: "deposits and a withdrawal, positive rate", flows: deposits, value: "1500",
			want: returns.Rate{Percent: decimal.RequireFromString("7.187891")},
		},
		{
			name: "deposits and a withdrawal, negative rate", flows: deposits, value: "1100",
			want: returns.Rate{Percent: decimal.RequireFromString("-7.750386")},
		},
		{
			name: "the order of the flows has no effect", flows: []returns.Flow{flow("-200", 100), flow("500", 400), flow("1000", 800)},
			value: "1500", want: returns.Rate{Percent: decimal.RequireFromString("7.187891")},
		},
		{
			name: "two years, exact", flows: []returns.Flow{flow("1000", 730)}, value: "1210",
			want: returns.Rate{Percent: decimal.RequireFromString("10")},
		},
		{name: "no flows", value: "100", want: returns.Rate{Reason: returns.ReasonNoFlows}},
		{
			name: "only money out has no sign change", flows: []returns.Flow{flow("-1000", 400)}, value: "500",
			want: returns.Rate{Reason: returns.ReasonNoRoot},
		},
		{
			name: "a negative value has no sign change", flows: []returns.Flow{flow("1000", 400)}, value: "-1",
			want: returns.Rate{Reason: returns.ReasonNoRoot},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := returns.Annual(tc.flows, decimal.RequireFromString(tc.value), true, today)

			assert.Equal(t, tc.want.Reason, got.Reason)
			assert.True(t, tc.want.Percent.Equal(got.Percent.Round(6)), "want %s, got %s", tc.want.Percent, got.Percent)
		})
	}
}

func TestSimple(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                     string
		totalResult, netDeposits string
		want                     returns.Rate
	}{
		{name: "positive", totalResult: "100", netDeposits: "1000", want: returns.Rate{Percent: decimal.RequireFromString("10")}},
		{name: "negative result", totalResult: "-1", netDeposits: "3", want: returns.Rate{Percent: decimal.RequireFromString("-33.3333333333333333")}},
		{name: "zero Net Deposits", totalResult: "100", netDeposits: "0", want: returns.Rate{Reason: returns.ReasonNoNetDeposits}},
		{name: "negative Net Deposits", totalResult: "100", netDeposits: "-1", want: returns.Rate{Reason: returns.ReasonNoNetDeposits}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := returns.Simple(decimal.RequireFromString(tc.totalResult), decimal.RequireFromString(tc.netDeposits))

			assert.Equal(t, tc.want.Reason, got.Reason)
			assert.True(t, tc.want.Percent.Equal(got.Percent), "want %s, got %s", tc.want.Percent, got.Percent)
		})
	}
}

// BenchmarkAnnual measures the Annual Return of 10 years of monthly deposits.
func BenchmarkAnnual(b *testing.B) {
	flows := make([]returns.Flow, 0, 120)
	for i := range 120 {
		flows = append(flows, flow("1000.00", 30*i+5))
	}
	value := decimal.RequireFromString("180000.00")
	for b.Loop() {
		returns.Annual(flows, value, true, today)
	}
}
