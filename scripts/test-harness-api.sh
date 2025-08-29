#!/bin/bash

# Test Harness API authentication
# Usage: ./test-harness-api.sh "YOUR_PAT_TOKEN" "YOUR_ACCOUNT_ID"

PAT_TOKEN="$1"
ACCOUNT_ID="$2"

if [ -z "$PAT_TOKEN" ] || [ -z "$ACCOUNT_ID" ]; then
    echo "Usage: $0 <PAT_TOKEN> <ACCOUNT_ID>"
    echo "Example: $0 'pat.ACCOUNT_ID.xxxxx' 'ACCOUNT_ID'"
    exit 1
fi

echo "Testing Harness API authentication..."
echo "PAT Token prefix: ${PAT_TOKEN:0:20}..."
echo "Account ID: $ACCOUNT_ID"
echo ""

# Test the currentUser endpoint with account ID as query parameter
echo "Testing with accountIdentifier query parameter:"
curl -s -X GET "https://app.harness.io/gateway/ng/api/user/currentUser?accountIdentifier=$ACCOUNT_ID" \
    -H "x-api-key: $PAT_TOKEN" \
    -H "Content-Type: application/json" \
    -w "\nHTTP Status: %{http_code}\n"

echo ""
echo "Testing without accountIdentifier query parameter:"
curl -s -X GET "https://app.harness.io/gateway/ng/api/user/currentUser" \
    -H "x-api-key: $PAT_TOKEN" \
    -H "Content-Type: application/json" \
    -w "\nHTTP Status: %{http_code}\n"