package payment

import (
	"embed"
	"html/template"
	"io"
	"sync"
)

//go:embed templates/*.html
var templateFS embed.FS

// TemplateRenderer handles HTML template rendering for Browser Post responses
type TemplateRenderer struct {
	templates map[string]*template.Template
	mu        sync.RWMutex
}

// TemplateName identifies which template to render
type TemplateName string

const (
	TemplateReceipt                 TemplateName = "receipt"
	TemplatePaymentMethodCreditCard TemplateName = "payment_method_credit_card"
	TemplatePaymentMethodBankAccount TemplateName = "payment_method_bank_account"
	TemplateError                   TemplateName = "error"
)

// NewTemplateRenderer creates a new template renderer with parsed templates
func NewTemplateRenderer() (*TemplateRenderer, error) {
	r := &TemplateRenderer{
		templates: make(map[string]*template.Template),
	}

	if err := r.loadTemplates(); err != nil {
		return nil, err
	}

	return r, nil
}

// loadTemplates parses all templates at startup
func (r *TemplateRenderer) loadTemplates() error {
	templateNames := []TemplateName{
		TemplateReceipt,
		TemplatePaymentMethodCreditCard,
		TemplatePaymentMethodBankAccount,
		TemplateError,
	}

	for _, name := range templateNames {
		tmpl, err := template.ParseFS(templateFS,
			"templates/base.html",
			"templates/"+string(name)+".html",
		)
		if err != nil {
			return err
		}
		r.templates[string(name)] = tmpl
	}

	return nil
}

// Render renders a template with the given data
func (r *TemplateRenderer) Render(w io.Writer, name TemplateName, data interface{}) error {
	r.mu.RLock()
	tmpl, ok := r.templates[string(name)]
	r.mu.RUnlock()

	if !ok {
		return ErrTemplateNotFound
	}

	return tmpl.ExecuteTemplate(w, "base", data)
}

// ErrTemplateNotFound is returned when a template is not found
var ErrTemplateNotFound = templateError("template not found")

type templateError string

func (e templateError) Error() string {
	return string(e)
}

// ReceiptData contains data for the receipt template
type ReceiptData struct {
	Approved     bool
	Amount       string
	Currency     string
	CardType     string
	MaskedCard   string
	AuthCode     string
	AuthRespText string
	TransactionID string
	TranNbr      string
	ReturnURL    string
}

// CreditCardData contains data for the credit card payment method template
type CreditCardData struct {
	CardBrand       string
	LastFour        string
	ExpirationDate  string
	PaymentMethodID string
	ReturnURL       string
}

// BankAccountData contains data for the bank account payment method template
type BankAccountData struct {
	AccountType     string
	LastFour        string
	PaymentMethodID string
	ReturnURL       string
}

// ErrorData contains data for the error template
type ErrorData struct {
	Message   string
	Details   string
	ReturnURL string
}
