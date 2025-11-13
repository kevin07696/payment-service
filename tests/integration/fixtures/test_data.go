package fixtures

import (
	"github.com/google/uuid"
)

// TestMerchantID is the UUID of the test merchant in the database
// This merchant is seeded during test setup
var TestMerchantID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
