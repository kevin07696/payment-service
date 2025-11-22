package pool

import (
	"sync"

	"github.com/kevin07696/payment-service/internal/adapters/ports"
)

// ServerPostRequestPool provides object pooling for EPX Server Post requests
// Hot path: Every payment transaction creates a ServerPostRequest
//
// Performance Impact:
// - At 1000 TPS: Saves 1000 allocations/sec
// - Typical ServerPostRequest: ~200 bytes + map overhead
// - Memory saved: ~200KB/sec at 1000 TPS
var ServerPostRequestPool = sync.Pool{
	New: func() interface{} {
		return &ports.ServerPostRequest{}
	},
}

// GetServerPostRequest retrieves a request from the pool
func GetServerPostRequest() *ports.ServerPostRequest {
	return ServerPostRequestPool.Get().(*ports.ServerPostRequest)
}

// PutServerPostRequest returns a request to the pool after resetting
func PutServerPostRequest(req *ports.ServerPostRequest) {
	if req == nil {
		return
	}
	// Reset to zero value to prevent data leakage
	*req = ports.ServerPostRequest{}
	ServerPostRequestPool.Put(req)
}

// ServerPostResponsePool provides object pooling for EPX Server Post responses
var ServerPostResponsePool = sync.Pool{
	New: func() interface{} {
		return &ports.ServerPostResponse{}
	},
}

// GetServerPostResponse retrieves a response from the pool
func GetServerPostResponse() *ports.ServerPostResponse {
	return ServerPostResponsePool.Get().(*ports.ServerPostResponse)
}

// PutServerPostResponse returns a response to the pool after resetting
func PutServerPostResponse(resp *ports.ServerPostResponse) {
	if resp == nil {
		return
	}
	*resp = ports.ServerPostResponse{}
	ServerPostResponsePool.Put(resp)
}

// BRICStorageRequestPool provides object pooling for BRIC storage requests
// Used when storing/retrieving payment methods (tokenization)
var BRICStorageRequestPool = sync.Pool{
	New: func() interface{} {
		return &ports.BRICStorageRequest{}
	},
}

// GetBRICStorageRequest retrieves a request from the pool
func GetBRICStorageRequest() *ports.BRICStorageRequest {
	return BRICStorageRequestPool.Get().(*ports.BRICStorageRequest)
}

// PutBRICStorageRequest returns a request to the pool after resetting
func PutBRICStorageRequest(req *ports.BRICStorageRequest) {
	if req == nil {
		return
	}
	*req = ports.BRICStorageRequest{}
	BRICStorageRequestPool.Put(req)
}

// BRICStorageResponsePool provides object pooling for BRIC storage responses
var BRICStorageResponsePool = sync.Pool{
	New: func() interface{} {
		return &ports.BRICStorageResponse{}
	},
}

// GetBRICStorageResponse retrieves a response from the pool
func GetBRICStorageResponse() *ports.BRICStorageResponse {
	return BRICStorageResponsePool.Get().(*ports.BRICStorageResponse)
}

// PutBRICStorageResponse returns a response to the pool after resetting
func PutBRICStorageResponse(resp *ports.BRICStorageResponse) {
	if resp == nil {
		return
	}
	*resp = ports.BRICStorageResponse{}
	BRICStorageResponsePool.Put(resp)
}
