package services

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService handles JWT generation and validation for payment receipts
type JWTService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// NewJWTService creates a new JWT service
func NewJWTService(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) *JWTService {
	return &JWTService{
		privateKey: privateKey,
		publicKey:  publicKey,
	}
}

// ReceiptClaims contains payment transaction data for POS receipt
type ReceiptClaims struct {
	TransactionID       string `json:"transaction_id"`
	Amount              string `json:"amount"`
	Currency            string `json:"currency"`
	Status              string `json:"status"`
	CardType            string `json:"card_type"`
	LastFour            string `json:"last_four"`
	AuthCode            string `json:"auth_code"`
	ExternalReferenceID string `json:"external_reference_id"` // POS order reference
	jwt.RegisteredClaims
}

// GenerateReceiptJWT creates a signed JWT with transaction receipt data
// This JWT is sent to POS in the redirect URL after payment processing
func (s *JWTService) GenerateReceiptJWT(txn *Transaction) (string, error) {
	if txn == nil {
		return "", fmt.Errorf("transaction cannot be nil")
	}

	claims := ReceiptClaims{
		TransactionID:       txn.ID,
		Amount:              txn.Amount,
		Currency:            txn.Currency,
		Status:              string(txn.Status),
		CardType:            txn.CardType,
		LastFour:            txn.LastFour,
		AuthCode:            txn.AuthCode,
		ExternalReferenceID: txn.ExternalReferenceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)), // Short expiry
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "payment-service",
			Subject:   txn.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signed, nil
}

// ValidateReceiptJWT validates and parses a receipt JWT
// POS uses this to verify the receipt came from Payment Service
func (s *JWTService) ValidateReceiptJWT(tokenString string) (*ReceiptClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&ReceiptClaims{},
		func(token *jwt.Token) (interface{}, error) {
			// Verify signing method
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return s.publicKey, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	if claims, ok := token.Claims.(*ReceiptClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid JWT claims")
}

// Transaction represents transaction data for JWT generation
// This is a subset of the full transaction model
type Transaction struct {
	ID                  string
	Amount              string
	Currency            string
	Status              TransactionStatus
	CardType            string
	LastFour            string
	AuthCode            string
	ExternalReferenceID string
}

// TransactionStatus represents transaction state
type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusRefunded  TransactionStatus = "refunded"
	TransactionStatusVoided    TransactionStatus = "voided"
)
