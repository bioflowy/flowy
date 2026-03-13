//go:build integration

// Package daemon — LED integration tests exercise localizeInputs and
// delocalizeOutputs directly against a real MinIO container started via
// testcontainers-go.
//
// This file lives in package daemon (internal test, not daemon_test) so it can
// call the unexported localizeInputs and delocalizeOutputs functions.
package daemon

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/minio"

	"github.com/bioflowy/flowy/internal/api"
	"github.com/bioflowy/flowy/internal/config"
	"github.com/bioflowy/flowy/internal/storage"
	"github.com/bioflowy/flowy/wdl/values"
)

const (
	ledTestUser     = "minioadmin"
	ledTestPassword = "minioadmin"
	ledTestBucketID = "flowy-led-integration-test"
	ledMinioImage   = "minio/minio:RELEASE.2024-11-07T00-52-28Z"
)

var (
	// ledTestEndpoint is the HTTP endpoint for the MinIO container,
	// e.g. "http://localhost:XXXXX". Set by TestMain.
	ledTestEndpoint string

	// ledTestBucket is the name of the S3 bucket created by TestMain.
	ledTestBucket string
)

// TestMain for the daemon package (integration-only) starts a MinIO container,
// creates the test bucket, and runs all tests. It is safe to define here
// because the existing daemon_test.go has no TestMain, and //go:build integration
// ensures this file is only compiled when -tags integration is passed.
func TestMain(m *testing.M) {
	os.Exit(runLEDTests(m))
}

// runLEDTests encapsulates the MinIO container lifecycle so that deferred
// cleanup runs correctly before os.Exit is called in TestMain.
func runLEDTests(m *testing.M) int {
	ctx := context.Background()

	mc, err := minio.Run(ctx, ledMinioImage,
		minio.WithUsername(ledTestUser),
		minio.WithPassword(ledTestPassword),
	)
	defer testcontainers.TerminateContainer(mc)
	if err != nil {
		log.Printf("start minio container for LED tests: %v", err)
		return 1
	}

	connStr, err := mc.ConnectionString(ctx)
	if err != nil {
		log.Printf("minio connection string: %v", err)
		return 1
	}
	ledTestEndpoint = "http://" + connStr
	ledTestBucket = ledTestBucketID

	// Create test bucket using aws-sdk-go-v2 S3 client.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(ledTestUser, ledTestPassword, ""),
		),
	)
	if err != nil {
		log.Printf("load AWS config: %v", err)
		return 1
	}
	s3c := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(ledTestEndpoint)
		o.UsePathStyle = true
	})
	if _, err := s3c.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(ledTestBucket),
	}); err != nil {
		log.Printf("create LED test bucket %q: %v", ledTestBucket, err)
		return 1
	}

	return m.Run()
}

// newLEDTestBackend constructs an S3Backend pointing at the shared MinIO container.
// AWS credentials are injected via t.Setenv so they are restored after the test.
func newLEDTestBackend(t *testing.T) storage.Backend {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", ledTestUser)
	t.Setenv("AWS_SECRET_ACCESS_KEY", ledTestPassword)

	cfg := config.Config{}
	cfg.Storage.Type = "s3"
	cfg.Storage.Bucket = ledTestBucket
	cfg.Storage.Endpoint = ledTestEndpoint
	cfg.Storage.Region = "us-east-1"
	cfg.Storage.MultipartThreshold = 100 * 1024 * 1024

	backend, err := storage.NewS3Backend(&cfg)
	if err != nil {
		t.Fatalf("NewS3Backend: %v", err)
	}
	return backend
}

// TestLEDLocalizeInputs verifies that localizeInputs downloads s3:// inputs from
// a real MinIO server and rewrites them to local file paths.
func TestLEDLocalizeInputs(t *testing.T) {
	backend := newLEDTestBackend(t)

	// Upload a test file to S3.
	const s3Key = "led-localize-test/input.txt"
	const inputContent = "localize input content"
	if err := backend.UploadFile(s3Key, []byte(inputContent)); err != nil {
		t.Fatalf("upload test input to S3: %v", err)
	}

	// Build inputs map with an s3:// URI referencing the uploaded file.
	s3URI := "s3://" + ledTestBucket + "/" + s3Key
	inputs := map[string]any{
		"input_file": s3URI,
	}

	localizedInputs, stageDir, err := localizeInputs(inputs, backend)
	if err != nil {
		t.Fatalf("localizeInputs: %v", err)
	}
	defer os.RemoveAll(stageDir)

	// stageDir must exist.
	if _, statErr := os.Stat(stageDir); os.IsNotExist(statErr) {
		t.Errorf("stageDir does not exist after localizeInputs: %q", stageDir)
	}

	// The localized input must be a local file path, not an s3:// URI.
	localPath, ok := localizedInputs["input_file"].(string)
	if !ok {
		t.Fatalf("localized input_file: got %T, want string", localizedInputs["input_file"])
	}
	if strings.HasPrefix(localPath, "s3://") {
		t.Errorf("localized path still has s3:// prefix: %q", localPath)
	}

	// The local file must be inside stageDir/inputs.
	inputsDir := filepath.Join(stageDir, "inputs")
	if !strings.HasPrefix(localPath, inputsDir) {
		t.Errorf("localized path %q is not under inputsDir %q", localPath, inputsDir)
	}

	// The local file must have the original content.
	got, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read localized file %q: %v", localPath, err)
	}
	if string(got) != inputContent {
		t.Errorf("localized file content: got %q, want %q", string(got), inputContent)
	}
}

// TestLEDDelocalizeOutputs verifies that delocalizeOutputs uploads files from a
// local workDir to S3 and returns s3:// URIs in the outputs map.
func TestLEDDelocalizeOutputs(t *testing.T) {
	backend := newLEDTestBackend(t)

	// Create a temp workDir with a mock output file.
	workDir := t.TempDir()
	const outputContent = "processed-output"
	outputFile := filepath.Join(workDir, "result.txt")
	if err := os.WriteFile(outputFile, []byte(outputContent), 0o644); err != nil {
		t.Fatalf("write mock output file: %v", err)
	}

	// Construct a claim with S3 output destination.
	claim := &api.TaskClaimResponse{
		RunID:         "led-delocalize-run",
		TaskName:      "testTask",
		ShardIndex:    0,
		OutputsDir:    "led-delocalize-test/outputs",
		StorageBucket: ledTestBucket,
		StorageType:   "s3",
	}

	// Build outputs map with a FileValue pointing at the local output file.
	outputs := map[string]values.Value{
		"result": values.FileValue{Value: outputFile},
	}

	jsonOutputs, err := delocalizeOutputs(outputs, workDir, claim, backend)
	if err != nil {
		t.Fatalf("delocalizeOutputs: %v", err)
	}

	// The result output must be an s3:// URI in the correct bucket.
	resultURI, ok := jsonOutputs["result"].(string)
	if !ok {
		t.Fatalf("result output: got %T, want string", jsonOutputs["result"])
	}
	if !strings.HasPrefix(resultURI, "s3://"+ledTestBucket+"/") {
		t.Errorf("result URI: got %q, want s3://%s/... prefix", resultURI, ledTestBucket)
	}

	// Parse the S3 key from the URI and download the file to verify content.
	_, s3Key, err := storage.ParseS3URI(resultURI)
	if err != nil {
		t.Fatalf("parse s3 URI %q: %v", resultURI, err)
	}
	got, err := backend.DownloadFile(s3Key)
	if err != nil {
		t.Fatalf("download result from S3 at key %q: %v", s3Key, err)
	}
	if string(got) != outputContent {
		t.Errorf("S3 result content: got %q, want %q", string(got), outputContent)
	}
}

// TestLEDFullRoundTrip exercises the complete LED (Localize-Execute-Delocalize)
// code path against a real MinIO server using an in-process mock executor.
//
// Steps:
//  1. Upload an input file to S3.
//  2. Call localizeInputs — downloads input from S3 to stageDir/inputs.
//  3. Mock executor: read localized input, write "processed: <content>" to workDir.
//  4. Call delocalizeOutputs — uploads workDir files to S3, returns s3:// URIs.
//  5. Download output from S3 and verify content is "processed: <original content>".
func TestLEDFullRoundTrip(t *testing.T) {
	backend := newLEDTestBackend(t)

	// Step 1: Upload input to S3.
	const inputContent = "raw input data"
	const inputKey = "led-full-roundtrip/input/data.txt"
	if err := backend.UploadFile(inputKey, []byte(inputContent)); err != nil {
		t.Fatalf("upload input to S3: %v", err)
	}

	// Step 2: Localize inputs (S3 -> local stageDir).
	s3URI := "s3://" + ledTestBucket + "/" + inputKey
	inputs := map[string]any{
		"data_file": s3URI,
	}
	localizedInputs, stageDir, err := localizeInputs(inputs, backend)
	if err != nil {
		t.Fatalf("localizeInputs: %v", err)
	}
	defer os.RemoveAll(stageDir)

	// Verify the localized input file path exists and has the expected content.
	localInputPath, ok := localizedInputs["data_file"].(string)
	if !ok {
		t.Fatalf("localized data_file: got %T, want string", localizedInputs["data_file"])
	}
	localInputData, err := os.ReadFile(localInputPath)
	if err != nil {
		t.Fatalf("read localized input: %v", err)
	}
	if string(localInputData) != inputContent {
		t.Errorf("localized input content: got %q, want %q", string(localInputData), inputContent)
	}

	// Step 3: Mock executor — reads localized input, writes processed output to workDir.
	// workDir is a subdirectory of stageDir (mimics task execution directory).
	workDir := filepath.Join(stageDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create workDir: %v", err)
	}
	processedContent := "processed: " + string(localInputData)
	outputFile := filepath.Join(workDir, "output.txt")
	if err := os.WriteFile(outputFile, []byte(processedContent), 0o644); err != nil {
		t.Fatalf("mock executor: write output file: %v", err)
	}

	// Step 4: Delocalize outputs (local workDir -> S3).
	claim := &api.TaskClaimResponse{
		RunID:         "led-full-roundtrip-run",
		TaskName:      "roundtripTask",
		ShardIndex:    0,
		OutputsDir:    "led-full-roundtrip/outputs",
		StorageBucket: ledTestBucket,
		StorageType:   "s3",
	}
	wdlOutputs := map[string]values.Value{
		"output_file": values.FileValue{Value: outputFile},
	}
	jsonOutputs, err := delocalizeOutputs(wdlOutputs, workDir, claim, backend)
	if err != nil {
		t.Fatalf("delocalizeOutputs: %v", err)
	}

	// Step 5: Download the S3 output and verify content.
	outputURI, ok := jsonOutputs["output_file"].(string)
	if !ok {
		t.Fatalf("output_file in jsonOutputs: got %T, want string", jsonOutputs["output_file"])
	}
	if !strings.HasPrefix(outputURI, "s3://") {
		t.Errorf("output_file URI: got %q, want s3:// prefix", outputURI)
	}

	_, s3OutputKey, err := storage.ParseS3URI(outputURI)
	if err != nil {
		t.Fatalf("parse output URI %q: %v", outputURI, err)
	}
	gotOutput, err := backend.DownloadFile(s3OutputKey)
	if err != nil {
		t.Fatalf("download output from S3 at key %q: %v", s3OutputKey, err)
	}
	if string(gotOutput) != processedContent {
		t.Errorf("S3 output content: got %q, want %q", string(gotOutput), processedContent)
	}
}
