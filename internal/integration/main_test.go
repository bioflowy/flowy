//go:build integration

// Package integration holds integration tests that spin up a real MinIO
// container via testcontainers-go and validate the S3 storage workflow.
// Run with: go test -tags integration -timeout 2m ./internal/integration/...
package integration

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"
)

const (
	testBucket   = "flowy-integration-test"
	testUser     = "minioadmin"
	testPassword = "minioadmin"
	minioImage   = "minio/minio:RELEASE.2024-11-07T00-52-28Z"
)

var (
	// minioEndpoint is the HTTP endpoint for the MinIO container, e.g. "http://localhost:XXXXX".
	// Set by TestMain and shared across all test functions in this package.
	minioEndpoint string

	// s3Client is a reusable AWS SDK v2 S3 client configured against the MinIO container.
	// Set by TestMain.
	s3Client *s3.Client
)

// TestMain starts a MinIO container, creates the test bucket, and runs all tests.
// It uses the runTests helper to enable proper defer-based cleanup (avoids Pitfall 6:
// os.Exit in TestMain skipping defers).
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests encapsulates the MinIO container lifecycle so that deferred cleanup
// runs correctly before os.Exit is called in TestMain.
func runTests(m *testing.M) int {
	ctx := context.Background()

	mc, err := minio.Run(ctx, minioImage,
		minio.WithUsername(testUser),
		minio.WithPassword(testPassword),
	)
	// Defer termination before checking err so that a partially-started container
	// is still cleaned up on failure.
	defer testcontainers.TerminateContainer(mc)
	if err != nil {
		log.Printf("start minio container: %v", err)
		return 1
	}

	// ConnectionString returns "host:port" without a scheme.
	// Use ephemeral port to avoid Pitfall 3 (hardcoded port 9000 conflicts).
	connStr, err := mc.ConnectionString(ctx)
	if err != nil {
		log.Printf("minio connection string: %v", err)
		return 1
	}
	minioEndpoint = "http://" + connStr

	// Build a dedicated S3 client with static credentials pointing to MinIO.
	// Use credentials.NewStaticCredentialsProvider to avoid Pitfall 1
	// (real AWS credentials leaking into integration tests).
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(testUser, testPassword, ""),
		),
	)
	if err != nil {
		log.Printf("load AWS config: %v", err)
		return 1
	}
	s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(minioEndpoint)
		o.UsePathStyle = true // required for MinIO path-style addressing
	})

	// Create the shared test bucket before any test runs.
	if _, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(testBucket),
	}); err != nil {
		log.Printf("create test bucket %q: %v", testBucket, err)
		return 1
	}

	return m.Run()
}
