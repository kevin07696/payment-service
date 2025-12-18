package merchant

import (
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kevin07696/payment-service/internal/domain"
	merchantv1 "github.com/kevin07696/payment-service/proto/merchant/v1"
)

// Validation helpers

func validateRegisterMerchantRequest(req *merchantv1.RegisterMerchantRequest) error {
	if req.MerchantId == "" {
		return fmt.Errorf("merchant_id is required")
	}
	if req.MacSecret == "" {
		return fmt.Errorf("mac_secret is required")
	}
	if req.CustNbr == "" {
		return fmt.Errorf("cust_nbr is required")
	}
	if req.MerchNbr == "" {
		return fmt.Errorf("merch_nbr is required")
	}
	if req.DbaNbr == "" {
		return fmt.Errorf("dba_nbr is required")
	}
	if req.TerminalNbr == "" {
		return fmt.Errorf("terminal_nbr is required")
	}
	if req.Environment == merchantv1.Environment_ENVIRONMENT_UNSPECIFIED {
		return fmt.Errorf("environment is required")
	}
	return nil
}

// Conversion helpers

func merchantToResponse(merchant *domain.Merchant) *merchantv1.MerchantResponse {
	return &merchantv1.MerchantResponse{
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		CustNbr:       merchant.CustNbr,
		MerchNbr:      merchant.MerchNbr,
		DbaNbr:        merchant.DBAnbr,
		TerminalNbr:   merchant.TerminalNbr,
		Environment:   environmentToProto(merchant.Environment),
		IsActive:      merchant.IsActive,
		CreatedAt:     timestamppb.New(merchant.CreatedAt),
		UpdatedAt:     timestamppb.New(merchant.UpdatedAt),
	}
}

func merchantToProto(merchant *domain.Merchant) *merchantv1.Merchant {
	return &merchantv1.Merchant{
		Id:            merchant.ID,
		MerchantId:    merchant.AgentID,
		MacSecretPath: merchant.MACSecretPath,
		CustNbr:       merchant.CustNbr,
		MerchNbr:      merchant.MerchNbr,
		DbaNbr:        merchant.DBAnbr,
		TerminalNbr:   merchant.TerminalNbr,
		Environment:   environmentToProto(merchant.Environment),
		IsActive:      merchant.IsActive,
		CreatedAt:     timestamppb.New(merchant.CreatedAt),
		UpdatedAt:     timestamppb.New(merchant.UpdatedAt),
		Metadata:      nil, // Not storing metadata yet
	}
}

func merchantToSummary(merchant *domain.Merchant) *merchantv1.MerchantSummary {
	return &merchantv1.MerchantSummary{
		MerchantId:  merchant.AgentID,
		MerchNbr:    merchant.MerchNbr,
		Environment: environmentToProto(merchant.Environment),
		IsActive:    merchant.IsActive,
		CreatedAt:   timestamppb.New(merchant.CreatedAt),
	}
}

func environmentToProto(env domain.Environment) merchantv1.Environment {
	switch env {
	case domain.EnvironmentSandbox:
		return merchantv1.Environment_ENVIRONMENT_SANDBOX
	case domain.EnvironmentProduction:
		return merchantv1.Environment_ENVIRONMENT_PRODUCTION
	default:
		return merchantv1.Environment_ENVIRONMENT_UNSPECIFIED
	}
}

func environmentFromProto(env merchantv1.Environment) domain.Environment {
	switch env {
	case merchantv1.Environment_ENVIRONMENT_SANDBOX:
		return domain.EnvironmentSandbox
	case merchantv1.Environment_ENVIRONMENT_PRODUCTION:
		return domain.EnvironmentProduction
	default:
		return domain.EnvironmentSandbox // Default
	}
}
