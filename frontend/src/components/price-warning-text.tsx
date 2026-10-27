import type { PriceWarning } from "@/lib/api"
import { formatPercent } from "@/lib/format"

// PriceWarningText gives the sentence of a Price Warning for the user, from its kind and its fields.
// The list of Price Warnings and the price source dialog both use it.
export function PriceWarningText({ warning }: { warning: PriceWarning }) {
  const ticker = <span className="num">{warning.priceTicker}</span>
  switch (warning.kind) {
    case "noData":
      return (
        <>
          The price source has no data for the price ticker {ticker}. Check the
          price ticker.
        </>
      )
    case "differentCurrency":
      return (
        <>
          The price source gives the price of {ticker} in{" "}
          <span className="num">{warning.sourceCurrency ?? "—"}</span>, but{" "}
          {warning.quoteCurrency === null ? (
            "the tool does not know the Quote Currency"
          ) : (
            <>
              the Quote Currency is{" "}
              <span className="num">{warning.quoteCurrency}</span>
            </>
          )}
          . Check the Quote Currency.
        </>
      )
    case "priceMismatch":
      return (
        <>
          The prices of the price source do not agree with the XTB prices.
          {warning.medianDifference !== null &&
            warning.maxDifference !== null && (
              <>
                {" "}
                The median difference is{" "}
                <span className="num">
                  {formatPercent(warning.medianDifference)}
                </span>
                , and the largest difference is{" "}
                <span className="num">
                  {formatPercent(warning.maxDifference)}
                </span>
                .
              </>
            )}{" "}
          Check the price ticker and the Quote Currency.
        </>
      )
  }
}
