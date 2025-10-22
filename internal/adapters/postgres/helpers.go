package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// nullText creates a pgtype.Text with empty string handling
func nullText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// pgNumericToDecimal converts pgtype.Numeric to decimal.Decimal
func pgNumericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	var dec decimal.Decimal
	str, err := n.MarshalJSON()
	if err != nil {
		return dec, fmt.Errorf("marshal numeric: %w", err)
	}
	// Remove quotes from JSON string
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}
	return decimal.NewFromString(string(str))
}
