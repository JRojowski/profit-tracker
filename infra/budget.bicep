// The cost budget of the resource group rg-profit-tracker. See ADR 0006.
// The setup script applies this template one time. The deployment workflow does not apply it,
// because Azure accepts a past start date only in the current month.

@description('The first day of the month when the budget starts, for example 2026-10-01.')
param startDate string

@description('The e-mail address that gets the alerts.')
param email string

@description('The monthly amount, in the billing currency of the subscription (PLN).')
param amount int = 20

var thresholds = [50, 80, 100]

resource budget 'Microsoft.Consumption/budgets@2026-06-01' = {
  name: 'budget-profit-tracker'
  properties: {
    category: 'Cost'
    amount: amount
    timeGrain: 'Monthly'
    timePeriod: {
      startDate: startDate
    }
    notifications: toObject(
      thresholds,
      t => 'actual-${t}',
      t => {
        enabled: true
        operator: 'GreaterThanOrEqualTo'
        threshold: t
        thresholdType: 'Actual'
        contactEmails: [email]
      }
    )
  }
}
