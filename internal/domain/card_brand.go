package domain

import "strings"

// CardBrand represents a credit card brand
type CardBrand string

const (
	CardBrandVisa            CardBrand = "Visa"
	CardBrandMastercard      CardBrand = "Mastercard"
	CardBrandAmericanExpress CardBrand = "American Express"
	CardBrandDiscover        CardBrand = "Discover"
	CardBrandJCB             CardBrand = "JCB"
	CardBrandUnknown         CardBrand = "Unknown"
)

// epxCardBrandMap maps EPX single-letter card type codes to CardBrand
var epxCardBrandMap = map[string]CardBrand{
	"V": CardBrandVisa,
	"M": CardBrandMastercard,
	"A": CardBrandAmericanExpress,
	"D": CardBrandDiscover,
	"J": CardBrandJCB,
}

// CardBrandFromEPXCode converts an EPX card type code to a CardBrand
// EPX returns single-letter codes: V (Visa), M (Mastercard), A (Amex), D (Discover), J (JCB)
func CardBrandFromEPXCode(code string) CardBrand {
	if brand, ok := epxCardBrandMap[strings.ToUpper(code)]; ok {
		return brand
	}
	// If unknown code, return the code itself as a fallback
	if code != "" {
		return CardBrand(code)
	}
	return CardBrandUnknown
}

// String returns the string representation of the card brand
func (cb CardBrand) String() string {
	return string(cb)
}

// IsKnown returns true if this is a recognized card brand
func (cb CardBrand) IsKnown() bool {
	switch cb {
	case CardBrandVisa, CardBrandMastercard, CardBrandAmericanExpress, CardBrandDiscover, CardBrandJCB:
		return true
	default:
		return false
	}
}
