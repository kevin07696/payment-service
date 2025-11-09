package browserpost

import (
	"fmt"
	"net/http"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
	"github.com/kevin07696/payment-service/internal/domain"
	"github.com/kevin07696/payment-service/internal/services"
	"go.uber.org/zap"
)

// CallbackHandler handles EPX browser post callbacks
// After customer completes payment on EPX, browser redirects here with payment result
type CallbackHandler struct {
	browserPostAdapter ports.BrowserPostAdapter
	jwtService         *services.JWTService
	transactionRepo    TransactionRepository
	logger             *zap.Logger
}

// TransactionRepository defines persistence operations for transactions
type TransactionRepository interface {
	// GetByTranNbr retrieves transaction by EPX transaction number
	GetByTranNbr(tranNbr string) (*domain.Transaction, error)

	// Update updates an existing transaction
	Update(txn *domain.Transaction) error
}

// NewCallbackHandler creates a new callback handler
func NewCallbackHandler(
	browserPostAdapter ports.BrowserPostAdapter,
	jwtService *services.JWTService,
	transactionRepo TransactionRepository,
	logger *zap.Logger,
) *CallbackHandler {
	return &CallbackHandler{
		browserPostAdapter: browserPostAdapter,
		jwtService:         jwtService,
		transactionRepo:    transactionRepo,
		logger:             logger,
	}
}

// HandleCallback processes EPX redirect after payment
// GET /api/v1/payments/callback?auth_guid=...&auth_resp=...&tran_nbr=...
func (h *CallbackHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	queryParams := r.URL.Query()
	h.logger.Info("received EPX callback",
		zap.Any("params", queryParams),
	)

	// Parse and validate EPX response
	epxResponse, err := h.browserPostAdapter.ParseRedirectResponse(queryParams)
	if err != nil {
		h.logger.Error("failed to parse EPX response",
			zap.Error(err),
			zap.Any("params", queryParams),
		)
		h.renderErrorPage(w, "Invalid payment response")
		return
	}

	// Retrieve transaction by TranNbr
	txn, err := h.transactionRepo.GetByTranNbr(epxResponse.TranNbr)
	if err != nil {
		h.logger.Error("failed to retrieve transaction",
			zap.Error(err),
			zap.String("tran_nbr", epxResponse.TranNbr),
		)
		h.renderErrorPage(w, "Transaction not found")
		return
	}

	// Update transaction with EPX response
	h.updateTransactionFromResponse(txn, epxResponse)

	// Save updated transaction
	if err := h.transactionRepo.Update(txn); err != nil {
		h.logger.Error("failed to update transaction",
			zap.Error(err),
			zap.String("transaction_id", txn.ID),
		)
		h.renderErrorPage(w, "Failed to process payment")
		return
	}

	// Generate receipt JWT for POS
	receiptJWT, err := h.jwtService.GenerateReceiptJWT(&services.Transaction{
		ID:                  txn.ID,
		Amount:              txn.Amount.String(),
		Currency:            txn.Currency,
		Status:              services.TransactionStatus(txn.Status),
		CardType:            h.safeString(txn.AuthCardType),
		LastFour:            h.extractLastFour(txn),
		AuthCode:            h.safeString(txn.AuthCode),
		ExternalReferenceID: h.safeString(txn.ExternalReferenceID),
	})
	if err != nil {
		h.logger.Error("failed to generate receipt JWT",
			zap.Error(err),
			zap.String("transaction_id", txn.ID),
		)
		h.renderErrorPage(w, "Failed to generate receipt")
		return
	}

	// Get return URL from transaction
	returnURL := h.safeString(txn.ReturnURL)
	if returnURL == "" {
		h.logger.Error("missing return URL",
			zap.String("transaction_id", txn.ID),
		)
		h.renderErrorPage(w, "Invalid configuration")
		return
	}

	// Build redirect URL with receipt JWT
	redirectURL := fmt.Sprintf("%s?receipt=%s", returnURL, receiptJWT)

	h.logger.Info("redirecting to POS",
		zap.String("transaction_id", txn.ID),
		zap.String("status", string(txn.Status)),
		zap.String("return_url", returnURL),
	)

	// Render HTML redirect page
	h.renderRedirectPage(w, redirectURL)
}

// updateTransactionFromResponse updates transaction with EPX callback data
func (h *CallbackHandler) updateTransactionFromResponse(txn *domain.Transaction, resp *ports.BrowserPostResponse) {
	// Update EPX fields
	txn.AuthGUID = &resp.AuthGUID
	txn.AuthResp = &resp.AuthResp
	txn.AuthCode = &resp.AuthCode
	txn.AuthRespText = &resp.AuthRespText
	txn.AuthCardType = &resp.AuthCardType
	txn.AuthAVS = &resp.AuthAVS
	txn.AuthCVV2 = &resp.AuthCVV2

	// Update status based on approval
	if resp.IsApproved {
		txn.Status = domain.TransactionStatusCompleted
	} else {
		txn.Status = domain.TransactionStatusFailed
	}

	h.logger.Info("updated transaction from EPX response",
		zap.String("transaction_id", txn.ID),
		zap.String("auth_guid", resp.AuthGUID),
		zap.String("auth_resp", resp.AuthResp),
		zap.Bool("is_approved", resp.IsApproved),
	)
}

// renderRedirectPage renders HTML that redirects browser to POS
func (h *CallbackHandler) renderRedirectPage(w http.ResponseWriter, redirectURL string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta http-equiv="refresh" content="0; url=%s">
	<title>Payment Processed</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			display: flex;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			text-align: center;
			padding: 2rem;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
		}
		.spinner {
			border: 4px solid #f3f3f3;
			border-top: 4px solid #3498db;
			border-radius: 50%%;
			width: 40px;
			height: 40px;
			animation: spin 1s linear infinite;
			margin: 0 auto 1rem;
		}
		@keyframes spin {
			0%% { transform: rotate(0deg); }
			100%% { transform: rotate(360deg); }
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="spinner"></div>
		<h2>Payment Processed</h2>
		<p>Returning to POS...</p>
		<p><a href="%s">Click here if not redirected automatically</a></p>
	</div>
	<script>
		setTimeout(function() {
			window.location.href = "%s";
		}, 100);
	</script>
</body>
</html>`, redirectURL, redirectURL, redirectURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}

// renderErrorPage renders an error page
func (h *CallbackHandler) renderErrorPage(w http.ResponseWriter, message string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Payment Error</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			display: flex;
			align-items: center;
			justify-content: center;
			height: 100vh;
			margin: 0;
			background: #f5f5f5;
		}
		.container {
			text-align: center;
			padding: 2rem;
			background: white;
			border-radius: 8px;
			box-shadow: 0 2px 8px rgba(0,0,0,0.1);
			max-width: 400px;
		}
		.error-icon {
			font-size: 48px;
			color: #e74c3c;
			margin-bottom: 1rem;
		}
		h2 {
			color: #e74c3c;
			margin: 0 0 1rem;
		}
	</style>
</head>
<body>
	<div class="container">
		<div class="error-icon">⚠️</div>
		<h2>Payment Error</h2>
		<p>%s</p>
		<p>Please contact support if this problem persists.</p>
	</div>
</body>
</html>`, message)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(html))
}

// safeString safely dereferences a string pointer
func (h *CallbackHandler) safeString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// extractLastFour extracts last 4 digits from AUTH_GUID (EPX BRIC format)
// AUTH_GUID format includes masked PAN, we extract last 4
func (h *CallbackHandler) extractLastFour(txn *domain.Transaction) string {
	// TODO: Parse AUTH_GUID BRIC format to extract last 4 digits
	// For now, return empty string
	// EPX BRIC format: AUTH_GUID contains encrypted card data
	// Need to parse the BRIC to get last 4
	return ""
}
