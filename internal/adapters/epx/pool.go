package epx

import (
	"net/url"
	"sync"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
)

var (
	// ServerPostRequestPool pools ServerPostRequest objects to reduce allocations
	// Used for every EPX Server Post API call (sale, auth, capture, etc.)
	ServerPostRequestPool = sync.Pool{
		New: func() interface{} {
			return &ports.ServerPostRequest{
				// Pre-allocate metadata map
				Metadata: make(map[string]string, 8),
			}
		},
	}

	// FormDataPool pools url.Values for form building
	// Used in Browser Post transactions to build POST form data
	FormDataPool = sync.Pool{
		New: func() interface{} {
			return make(url.Values, 16) // Pre-allocate capacity for typical form
		},
	}
)

// GetServerPostRequest retrieves a ServerPostRequest from the pool
func GetServerPostRequest() *ports.ServerPostRequest {
	return ServerPostRequestPool.Get().(*ports.ServerPostRequest)
}

// PutServerPostRequest returns a ServerPostRequest to the pool after clearing sensitive data
func PutServerPostRequest(req *ports.ServerPostRequest) {
	// Clear sensitive data before returning to pool
	req.CustNbr = ""
	req.MerchNbr = ""
	req.DBAnbr = ""
	req.TerminalNbr = ""
	req.TransactionType = ""
	req.Amount = ""
	req.PaymentType = ""
	req.AuthGUID = ""
	req.TranNbr = ""
	req.TranGroup = ""
	req.OriginalAuthGUID = ""
	req.OriginalAmount = ""
	req.CustomerID = ""

	// Clear pointer fields (sensitive data)
	req.AccountNumber = nil
	req.RoutingNumber = nil
	req.ExpirationDate = nil
	req.CVV = nil
	req.FirstName = nil
	req.LastName = nil
	req.Address = nil
	req.City = nil
	req.State = nil
	req.ZipCode = nil
	req.CardEntryMethod = nil
	req.IndustryType = nil
	req.ACIExt = nil
	req.StdEntryClass = nil
	req.ReceiverName = nil

	// Clear metadata map
	for k := range req.Metadata {
		delete(req.Metadata, k)
	}

	ServerPostRequestPool.Put(req)
}

// GetFormData retrieves a url.Values from the pool
func GetFormData() url.Values {
	formData := FormDataPool.Get().(url.Values)
	// Clear existing data
	for k := range formData {
		delete(formData, k)
	}
	return formData
}

// PutFormData returns a url.Values to the pool
func PutFormData(formData url.Values) {
	// Clear all data before returning to pool
	for k := range formData {
		delete(formData, k)
	}
	FormDataPool.Put(formData)
}
