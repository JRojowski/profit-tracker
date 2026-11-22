package importer

import (
	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// The values of the Kraken Account. The Kraken export has no account number, so the number is fixed.
const (
	KrakenAccountNumber = "kraken-spot"
	krakenAccountName   = "Kraken"
)

// krakenCategory is the category of a crypto Instrument.
const krakenCategory = "CRYPTO"

// parseKraken reads a Kraken ledger export. The Kraken path has no Lots and no Ledger checks.
// An unsupported entry stops the Import with a *RejectedError that wraps a *kraken.UnsupportedError.
func parseKraken(data []byte) (export, error) {
	file, err := kraken.Read(data)
	if err != nil {
		return export{}, &RejectedError{Err: err}
	}
	ops := make([]store.CashOperation, 0, len(file.CashOperations))
	for _, op := range file.CashOperations {
		saved := store.CashOperation{
			XTBID:  op.RefID,
			Time:   op.Time,
			Amount: op.Amount,
		}
		switch op.Kind {
		case kraken.KindDeposit:
			saved.Type = ledger.TypeDeposit
		case kraken.KindPurchase:
			saved.Type = ledger.TypeCryptoPurchase
			setCryptoInstrument(&saved, op.Asset)
			saved.Volume = decimal.NewNullDecimal(op.Volume)
			saved.Price = decimal.NewNullDecimal(op.Price)
			saved.Fee = decimal.NewNullDecimal(op.Fee)
		case kraken.KindReward:
			// The cost of a Reward is not in the export. The Ledger calculation takes it from the stored closes (ADR 0009).
			saved.Type = ledger.TypeReward
			setCryptoInstrument(&saved, op.Asset)
			saved.Volume = decimal.NewNullDecimal(op.Volume)
		}
		ops = append(ops, saved)
	}
	return export{
		Provider:          ProviderKraken,
		AccountNumber:     KrakenAccountNumber,
		SuggestedName:     krakenAccountName,
		SuggestedType:     TypeRegular,
		SuggestedCurrency: kraken.Currency,
		CashOperations:    ops,
	}, nil
}

// setCryptoInstrument sets the crypto Instrument of the asset on the Cash Operation.
func setCryptoInstrument(op *store.CashOperation, asset string) {
	op.Ticker = asset
	op.InstrumentName = asset
	op.Category = krakenCategory
	// The Ledger calculation has no cross rates, so the price ticker must quote the asset in EUR.
	op.QuoteCurrency = kraken.Currency
	op.DefaultPriceTicker = asset + "-" + kraken.Currency
	op.DefaultQuoteCurrency = kraken.Currency
}
