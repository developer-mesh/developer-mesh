//go:build integration

package aws

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSIntegration(t *testing.T) {
	// Skip test if integration tests are not enabled
	if os.Getenv("ENABLE_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping AWS integration tests - set ENABLE_INTEGRATION_TESTS=true to run")
	}

	// Create logger for AWS services
	logger := observability.NewLogger("aws-test")
	require.NotNil(t, logger)

	// Get AWS endpoint from environment or use default
	endpoint := os.Getenv("AWS_ENDPOINT_URL")
	if endpoint == "" {
		endpoint = "http://localhost:4566"
	}

	// Create a context with timeout for all AWS operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("AWS authentication and configuration integration", func(t *testing.T) {
		// Create AWS config with endpoint from environment
		authConfig := aws.AuthConfig{
			Region:   os.Getenv("AWS_REGION"),
			Endpoint: endpoint,
		}

		// Use test credentials if not specified in environment
		if authConfig.Region == "" {
			authConfig.Region = "us-east-1"
		}

		// Set test credentials if not provided
		if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
			t.Logf("Using test AWS credentials")
			// These will be used by LocalStack
			os.Setenv("AWS_ACCESS_KEY_ID", "test")
			os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
		}

		awsConfig, err := aws.GetAWSConfig(ctx, authConfig)

		// Fail test rather than skip - in CI we want this to work
		require.NoError(t, err, "Failed to get AWS config: %v", err)
		require.NotNil(t, awsConfig, "AWS config should not be nil")

		// Verify region was properly set
		assert.Equal(t, authConfig.Region, awsConfig.Region)
		t.Logf("Successfully created AWS config with endpoint %s", endpoint)
	})

	t.Run("AWS service client integration with observability", func(t *testing.T) {
		// This test verifies that AWS clients can be created with proper logging integration
		authConfig := aws.AuthConfig{
			Region:   os.Getenv("AWS_REGION"),
			Endpoint: endpoint,
		}

		if authConfig.Region == "" {
			authConfig.Region = "us-east-1"
		}

		mockConfig, err := aws.GetAWSConfig(ctx, authConfig)
		require.NoError(t, err, "Failed to get AWS config")

		// Create S3 client - this tests that the AWS package correctly integrates
		// with the AWS SDK and our observability stack
		s3Client := s3.NewFromConfig(mockConfig)
		require.NotNil(t, s3Client, "S3 client should not be nil")

		// Try a basic S3 operation to ensure the client works
		_, err = s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
		require.NoError(t, err, "Failed to list S3 buckets with localstack")
		t.Log("Successfully connected to S3 service")

		// Create RDS client through our package
		rdsClient, err := aws.NewExtendedRDSClient(ctx, aws.RDSConnectionConfig{
			// Set required fields for RDSConnectionConfig
			Host:     os.Getenv("DATABASE_HOST"),
			Port:     5432,
			Database: os.Getenv("DATABASE_NAME"),
			Username: os.Getenv("DATABASE_USER"),
			Password: os.Getenv("DATABASE_PASSWORD"),
			// Set Auth config nested inside RDSConnectionConfig
			Auth: authConfig,
		})

		require.NoError(t, err, "Failed to create RDS client")
		require.NotNil(t, rdsClient, "RDS client should not be nil")
		t.Log("Successfully created RDS client")

		// Test IRSA detection (this should use the mock path in test environment)
		isIRSA := aws.IsIRSAEnabled()
		t.Logf("IRSA enabled: %v", isIRSA)
	})
}
