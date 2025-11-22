//go:build integration
// +build integration

package wordpress

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	paymentv1connect "github.com/kevin07696/payment-service/proto/payment/v1/paymentv1connect"
)

const (
	wordpressURL  = "http://localhost:8082"
	paymentAPIURL = "http://localhost:8080"
	merchantID    = "00000000-0000-0000-0000-000000000001"
	serviceID     = "test-service-001"
	adminUser     = "admin"
	adminPass     = "admin"
)

const testPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA0/CXHurhUsdB7Z+LmdXoYTNULaQMEuC10xbPOUUtqy47wFX9
g/ZK9c/yFLYqER4A9vEiNuW2sn4KQsYZnJ/tbxM21JrjhgOGiC5HkmE1xHRzB6L9
JP16tLvheiedYjuVwcb3HnuTBeXlxNvL8yliWtrROTUz1WC0PicMLwlkUipkzXi3
0rrGgiaP6NbLNK3V7vmDCbNlZXmElIHN7NoVaTwYYPxJZOTnYe3PNO3/2+uQgkLJ
U4tynmt70q6lR/ttHGBGlEBJk0JRHEfXfZaThAD1eFUO7f9e6zRT+Dmw94L6WEyj
CRR5Rk3OTgN+2sFxTzUlVfXRgZmVjAilJlEoHwIDAQABAoIBABqxTHcqYeKJEfaZ
h32CgVfsnQd6h8LA5mWFk+fEnLHYitH4gotiM6Kt4/FT2Ax72OdBC2wall34ndY3
GPau9bptkxRHxawVOZZhLcZz08/AUtR9ZKCKBDBLEWTPJHVAx+W1513BdozhnYSj
ohYn+ikzMfKgjryrB0hkppYt+qKWVYgWvzIhKwF9lUH+4JyOqRGtVpXObHUQHYdI
4XaZGRIEPgEtFB7JYLFRlXEAE5UaN8iYc15mDq8L6odxr+vxDOo+EWMGK59u6m4W
ckIL4AoINqI1vuYvUpN6doO8Sc9qUsfLgBUaWMUW6LLaj2GQVWyx0dqwvvj1YdjC
BC7+ZtECgYEA3Wr0uoU/7wGAhvGXIfiNLaZmFvPt+nyIYnWgvFqXkLIEfbDKjtVc
RhX/OPj5562UWy7kuHKgpqN5AFQCKrTiiFENEiYKWvDHgOArDiMaOrZmU/LOPyG5
aDDUPg1lRAhEeygJTdwEOYEgU7026LYnhQKk7eu7Hmx2mj4f9uDfxh0CgYEA9Qqr
9ZVsiiFKFSMJvNqjXgXBoHcgt+feoT1isHKbWz6ms0U4U2h4VQob6FgxPQ14fcCI
rpiLm7+QvEZIAk8U5a9BuUQbDYnGXu+aA/T/6TaNRhNRDxI8ora+znO0zdXel3BU
9AR7cRobfxno3Xln5VHcADEkgMjzwF74t6snomsCgYBwq41vIIFBGO2TPXqfgcAt
e6A1i9kMfrRUDfFGB39a1Qtt/jmE51N2IplmH2PjnbOBluIybboMMeFP5m/X1YX0
wfG5y3u3fRC4Jtoh7oDZYZm+nC6Rd5LGTxqhnOVr8h0O4nehlBTeQjP2CLHZR1/i
0k6k9zCXsa/Em1peoV2djQKBgAU5LgM1JTQok4Cx14JMEtFtQ/xcrbjd23QKb/Ec
8EzYoAsQPawhfPcrGP8x6hLIF7pugTtfixJN2hL5WI2cC/D9dGQznHQEbNMXPmw5
K79X51kIDmFI3TwGsziJZOBCX9VQkq8E7XCywsVJ0xntfZZ40Ty7z3BjWDbQj3Ky
1kxzAoGBAME8+pbhXQ79/6zKNoNksI8Csggn93g3gflAF8+QEFwm4m0TWYyu1/Ep
goIDX/BHafmRsQnOjC7Qusl5xNDxmjjVMl0WeSWH5oiVFch2pcnXX1MqNj3AwQUN
lKWZaKdXH3Kj+1TNN6BdAmDtbdIQ+S+cyTEh5fHZ4f7sx8rPucuN
-----END RSA PRIVATE KEY-----`

// authTransport adds JWT auth to requests
type authTransport struct {
	token     string
	transport http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.transport.RoundTrip(req)
}

// generateJWT creates a JWT token for API authentication
func generateJWT() string {
	privateKey, _ := jwt.ParseRSAPrivateKeyFromPEM([]byte(testPrivateKey))

	claims := jwt.MapClaims{
		"iss":         serviceID,
		"merchant_id": merchantID,
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"jti":         uuid.New().String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(privateKey)
	return tokenString
}

// createPaymentClient creates an authenticated payment service client
func createPaymentClient() paymentv1connect.PaymentServiceClient {
	tokenString := generateJWT()

	httpClient := &http.Client{
		Transport: &authTransport{
			token:     tokenString,
			transport: http.DefaultTransport,
		},
	}

	return paymentv1connect.NewPaymentServiceClient(httpClient, paymentAPIURL)
}
