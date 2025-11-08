#!/bin/bash
# Test Bedrock Connection for Local Development
# This script verifies AWS Bedrock connectivity and model access

set -e

echo "========================================"
echo "Bedrock Connection Test"
echo "========================================"
echo ""

# Load environment variables
if [ -f .env ]; then
    export $(cat .env | grep -v '^#' | xargs)
fi

echo "1. Checking AWS credentials..."
if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo "❌ ERROR: AWS credentials not configured"
    echo "   Please set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY in .env"
    exit 1
fi
echo "✅ AWS credentials found"
echo ""

echo "2. Checking AWS region..."
if [ -z "$AWS_REGION" ]; then
    echo "❌ ERROR: AWS_REGION not set"
    exit 1
fi
echo "✅ AWS region: $AWS_REGION"
echo ""

echo "3. Checking Bedrock configuration..."
if [ "$BEDROCK_ENABLED" != "true" ]; then
    echo "❌ ERROR: BEDROCK_ENABLED is not set to 'true'"
    exit 1
fi
echo "✅ Bedrock enabled: $BEDROCK_ENABLED"
echo "✅ Default model: ${BEDROCK_DEFAULT_MODEL:-amazon.titan-embed-text-v2:0}"
echo ""

echo "4. Testing AWS CLI access (if available)..."
if command -v aws &> /dev/null; then
    echo "Checking AWS identity..."
    aws sts get-caller-identity --region $AWS_REGION 2>&1 || {
        echo "⚠️  WARNING: AWS CLI authentication failed"
        echo "   This might be a temporary issue or incorrect credentials"
    }
    echo ""

    echo "Listing available Bedrock models..."
    aws bedrock list-foundation-models \
        --region $AWS_REGION \
        --query 'modelSummaries[?contains(modelId, `embed`)].{ModelId:modelId,Provider:providerName}' \
        --output table 2>&1 || {
        echo "⚠️  WARNING: Could not list Bedrock models"
        echo "   Your account may need Bedrock access enabled"
        echo "   Visit: https://console.aws.amazon.com/bedrock"
    }
else
    echo "⚠️  AWS CLI not installed - skipping AWS-specific tests"
    echo "   Install AWS CLI: https://aws.amazon.com/cli/"
fi

echo ""
echo "5. Recommended models for local development:"
echo "   ┌──────────────────────────────────────┬──────────┬──────────────────┐"
echo "   │ Model                                │ Dims     │ Cost per 1K      │"
echo "   ├──────────────────────────────────────┼──────────┼──────────────────┤"
echo "   │ amazon.titan-embed-text-v2:0 (⭐)    │ 1024     │ ~\$0.0001 (best) │"
echo "   │ cohere.embed-english-v3              │ 1024     │ ~\$0.0001         │"
echo "   │ amazon.titan-embed-text-v1           │ 1536     │ ~\$0.0002         │"
echo "   │ cohere.embed-multilingual-v3         │ 1024     │ ~\$0.0001         │"
echo "   └──────────────────────────────────────┴──────────┴──────────────────┘"
echo ""
echo "   For production/high-accuracy needs:"
echo "   ┌──────────────────────────────────────┬──────────┬──────────────────┐"
echo "   │ anthropic.claude-3-haiku-*           │ 3072     │ ~\$0.00025        │"
echo "   │ anthropic.claude-3-sonnet-*          │ 4096     │ ~\$0.003          │"
echo "   │ meta.llama3-8b-embedding-v1:0        │ 4096     │ ~\$0.0002         │"
echo "   └──────────────────────────────────────┴──────────┴──────────────────┘"
echo ""

echo "========================================"
echo "Configuration Summary"
echo "========================================"
echo "AWS Region:        $AWS_REGION"
echo "Bedrock Enabled:   $BEDROCK_ENABLED"
echo "Default Model:     ${BEDROCK_DEFAULT_MODEL:-amazon.titan-embed-text-v2:0}"
echo "Embedding Enabled: ${EMBEDDING_ENABLED:-true}"
echo ""
echo "✅ Configuration looks good!"
echo ""
echo "Next steps:"
echo "  1. Ensure Bedrock access is enabled in AWS Console:"
echo "     https://console.aws.amazon.com/bedrock"
echo "  2. Request model access for your chosen models"
echo "  3. Start development: make dev"
echo "  4. Test embedding generation via API"
echo ""
