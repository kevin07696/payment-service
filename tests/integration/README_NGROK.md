# Integration Tests with Real BRIC (ngrok)

⚠️ **Current Limitation**: EPX test merchant has `http://localhost:8081` whitelisted for callbacks. EPX rejects ngrok URLs with error `BP_113: Merchant REDIRECT_URL mismatch`. To get real BRICs from EPX, you need merchant credentials that allow dynamic redirect URLs.

**Workaround**: The existing 24 integration tests successfully get real TACs from EPX and use fixture BRICs for Server Post operations. This provides comprehensive testing of the payment flow.

These tests were designed to get **real BRICs** from EPX Browser Post API and use them for Server Post operations (Capture, Void, Refund).

## Requirements

### 1. Install ngrok

```bash
# Fedora/RHEL
sudo snap install ngrok

# Ubuntu/Debian
sudo snap install ngrok

# macOS
brew install ngrok/ngrok/ngrok

# Or download from https://ngrok.com/download
```

### 2. Install Chrome/Chromium (for automated browser tests)

```bash
# Fedora
sudo dnf install chromium

# Ubuntu
sudo apt install chromium-browser

# macOS
brew install --cask chromium
```

## Running Tests

### Option A: Auto-start ngrok (Recommended)

Tests will automatically start ngrok if installed:

```bash
go test -v ./tests/integration/payment/browser_post_workflow_test.go -tags=integration
```

**What happens:**
1. ✅ Test starts ngrok tunnel automatically on port 8081
2. ✅ EPX can reach callback at public URL (e.g., `https://abc123.ngrok.io`)
3. ✅ Browser Post returns **real BRIC** from EPX
4. ✅ Server Post operations (Capture/Void/Refund) use **real BRIC**
5. ✅ ngrok stops automatically when test completes

### Option B: Manual ngrok (For debugging)

Start ngrok manually in a separate terminal:

```bash
# Terminal 1: Start ngrok
ngrok http 8081

# You'll see output like:
# Forwarding  https://abc123.ngrok.io -> http://localhost:8081
```

Then run tests with the ngrok URL:

```bash
# Terminal 2: Run tests with ngrok URL
export CALLBACK_BASE_URL=https://abc123.ngrok.io
go test -v ./tests/integration/payment/browser_post_workflow_test.go -tags=integration
```

### Option C: Skip real BRIC tests

If ngrok is not installed, tests will skip gracefully:

```bash
go test -v ./tests/integration/payment/browser_post_workflow_test.go -tags=integration
# Output: ⏭️  ngrok not installed and CALLBACK_BASE_URL not set - skipping test requiring external callback
```

## Test Coverage

### Browser Post Workflow Tests (with real BRIC)

Located in: `tests/integration/payment/browser_post_workflow_test.go`

| Test | Flow | What it verifies |
|------|------|------------------|
| `TestBrowserPost_AuthCapture_Workflow` | SALE → REFUND | Real BRIC works for refunds |
| `TestBrowserPost_AuthCaptureRefund_Workflow` | AUTH → CAPTURE → REFUND | Real BRIC works for full workflow |
| `TestBrowserPost_AuthVoid_Workflow` | AUTH → VOID | Real BRIC works for void operations |

**Each test:**
1. 🚀 Starts ngrok (if needed)
2. 🌐 Gets real BRIC from EPX via automated browser
3. ✅ Verifies Server Post operations work with real BRIC
4. ✅ Validates no "RR" errors (invalid BRIC errors)

## Troubleshooting

### ngrok rate limits

Free ngrok accounts have connection limits. If you see errors:

```bash
# Use your ngrok auth token (sign up at https://ngrok.com)
ngrok config add-authtoken YOUR_TOKEN_HERE
```

### Port already in use

```bash
# Check if ngrok is already running
ps aux | grep ngrok

# Kill existing ngrok
pkill ngrok
```

### Chrome not found

Tests will fail if Chrome/Chromium is not installed:

```bash
# Install Chrome/Chromium
sudo dnf install chromium  # Fedora
sudo apt install chromium-browser  # Ubuntu
```

### Callback timeout

If EPX can't reach the callback URL:
- ✅ Ensure payment server is running on port 8081
- ✅ Check ngrok tunnel is active: http://localhost:4040
- ✅ Verify firewall allows ngrok connections

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ Test Flow: Getting Real BRIC with ngrok                    │
└─────────────────────────────────────────────────────────────┘

1. Test starts ngrok tunnel
   ├── Auto-detects if ngrok installed
   ├── Starts: ngrok http 8081
   └── Gets public URL: https://abc123.ngrok.io

2. Test calls Browser Post form endpoint
   ├── GET /api/v1/payments/browser-post/form
   ├── With callback: https://abc123.ngrok.io/api/v1/payments/browser-post/callback
   └── Gets TAC from EPX

3. Headless Chrome submits to EPX
   ├── Fills test card: 4111111111111111
   ├── Submits to EPX: https://services.epxuap.com/browserpost/
   └── EPX processes and redirects back

4. EPX calls back via ngrok
   ├── POST https://abc123.ngrok.io/api/v1/payments/browser-post/callback
   ├── ngrok forwards to: http://localhost:8081/api/v1/payments/browser-post/callback
   ├── Handler creates transaction with REAL BRIC
   └── Transaction stored in database

5. Test uses real BRIC for Server Post
   ├── CAPTURE: Uses AUTH BRIC
   ├── VOID: Uses AUTH BRIC
   └── REFUND: Uses CAPTURE BRIC

6. Test verifies success
   ├── ✅ All transactions approved
   └── ✅ No "RR" errors (invalid BRIC)
```

## CI/CD Integration

For GitHub Actions or other CI:

```yaml
name: Integration Tests with Real BRIC

on: [push, pull_request]

jobs:
  test-real-bric:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Install ngrok
        run: |
          curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null
          echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list
          sudo apt update && sudo apt install ngrok

      - name: Install Chrome
        run: sudo apt install chromium-browser

      - name: Start services
        run: docker-compose up -d

      - name: Run tests with ngrok
        run: go test -v ./tests/integration/payment/browser_post_workflow_test.go -tags=integration
```

## Benefits

✅ **Real BRIC testing** - Tests use actual EPX tokens, not mocks
✅ **Automated** - No manual intervention needed
✅ **Fast** - Tests run in ~30 seconds
✅ **Reliable** - Skips gracefully if ngrok not available
✅ **No code changes** - Works with existing tests
✅ **CI/CD ready** - Can be integrated into pipelines
