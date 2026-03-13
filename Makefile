FLOWY_BIN     := ./flowy
SERVER_BIN    := ./flowy-server
DAEMON_BIN    := ./flowy-daemon
SPECTOOL_BIN  := $(HOME)/.cargo/bin/spectool
WDL_SPEC_DIR  := /tmp/wdl-spec

CARGO         := $(HOME)/.cargo/bin/cargo

# Comma-separated list of test name substrings to skip.
# Loaded from spectool-exclude.txt (strips comments and blank lines).
EXCLUDE_PATTERNS := $(shell grep -v '^\s*\#' spectool-exclude.txt 2>/dev/null | grep -v '^\s*$$' | paste -sd ',' -)

# PID files for server/daemon lifecycle management.
SERVER_PID := .server.pid
DAEMON_PID := .daemon.pid

# MinIO container and S3 integration variables.
MINIO_IMAGE  := minio/minio:RELEASE.2024-11-07T00-52-28Z
MINIO_PORT   := 9000
MINIO_CID    := .minio.cid
TEST_BUCKET  := flowy-spectool-test
S3_CONFIG    := /tmp/flowy-s3.yaml
S3_PORT      := 8125

.PHONY: build spectool spectool-s3 integration test clean

# --- Build targets ---

build: $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN)

GO_SOURCES := $(shell find cmd wdl internal -name '*.go' 2>/dev/null)

$(FLOWY_BIN): $(GO_SOURCES)
	go build -o $(FLOWY_BIN) ./cmd/flowy

$(SERVER_BIN): $(GO_SOURCES)
	go build -o $(SERVER_BIN) ./cmd/flowy-server

$(DAEMON_BIN): $(GO_SOURCES)
	go build -o $(DAEMON_BIN) ./cmd/flowy-daemon

test:
	go test ./... -count=1

# --- spectool installation ---

$(SPECTOOL_BIN):
	@echo "Installing spectool..."
	@rm -rf /tmp/spectool-src
	git clone --branch main --depth 1 https://github.com/openwdl/spectool /tmp/spectool-src
	cd /tmp/spectool-src && \
		sed -i 's/git2 = "0.20.0"/git2 = { version = "0.20.0", features = ["vendored-openssl"] }/' Cargo.toml && \
		$(CARGO) install --path .
	@rm -rf /tmp/spectool-src

# --- WDL 1.2 spec clone ---

$(WDL_SPEC_DIR):
	git clone --branch wdl-1.2 --depth 1 https://github.com/openwdl/wdl $(WDL_SPEC_DIR)

# --- Conformance test run ---
#
# spectool separates workflow-target and task-target arguments:
#   --workflow-target-args: appended when running a workflow (no --task needed)
#   --task-target-args:     appended when running a task (use --task ~{target})
#
# --redirect-stdout: appends "> ~{output}" to the command so the engine's stdout
#                   JSON is captured as the outputs file. flowy prints JSON to stdout
#                   on success, making this the natural integration point.
#
# Exclude patterns are read from spectool-exclude.txt (one pattern per line,
# comments allowed). They are passed as a comma-separated list to --exclude.
#
# The target starts flowy-server and flowy-daemon before running tests,
# and stops them afterward (even if spectool fails).

spectool: $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
	@# Clean up stale data from previous runs and recreate base directory.
	@# Use Docker to remove root-owned files created by task containers.
	@docker run --rm -v $(abspath .)/flowy-data:/data alpine sh -c "rm -rf /data/*" 2>/dev/null || true
	@rm -rf ./flowy-data
	@mkdir -p ./flowy-data
	@# Start flowy-server in the background.
	$(SERVER_BIN) &  echo $$! > $(SERVER_PID)
	@echo "Waiting for flowy-server to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s -o /dev/null http://localhost:8124/runs 2>/dev/null; then \
			echo "flowy-server is ready."; \
			break; \
		fi; \
		if [ $$i -eq 10 ]; then \
			echo "ERROR: flowy-server failed to start"; \
			kill `cat $(SERVER_PID)` 2>/dev/null; rm -f $(SERVER_PID); \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@# Start flowy-daemon in the background.
	$(DAEMON_BIN) --workers 4 &  echo $$! > $(DAEMON_PID)
	@sleep 1
	@echo "flowy-daemon started."
	@# Run spectool tests; capture exit code so we always clean up.
	$(eval EXCL := $(EXCLUDE_PATTERNS))
	$(SPECTOOL_BIN) test \
		"$(abspath $(FLOWY_BIN)) run --input ~{input} ~{path}" \
		--redirect-stdout \
		--all-capabilities \
		--workflow-target-args="" \
		--task-target-args="--task ~{target}" \
		-s $(WDL_SPEC_DIR) \
		-c ./flowy-data/spectool \
		$(if $(EXCL),--exclude "$(EXCL)",) \
	; SPECTOOL_EXIT=$$?; \
	echo "Stopping flowy-daemon (PID $$(cat $(DAEMON_PID)))..."; \
	kill $$(cat $(DAEMON_PID)) 2>/dev/null; rm -f $(DAEMON_PID); \
	echo "Stopping flowy-server (PID $$(cat $(SERVER_PID)))..."; \
	kill $$(cat $(SERVER_PID)) 2>/dev/null; rm -f $(SERVER_PID); \
	wait; \
	exit $$SPECTOOL_EXIT

# --- Integration tests ---
#
# Runs Go integration tests tagged with `integration` against a real MinIO
# testcontainer. The tests manage their own container lifecycle internally.

integration:
	go test -tags integration -timeout 2m -count=1 ./internal/integration/...

# --- Conformance test run with S3/MinIO backend ---
#
# spectool-s3 mirrors the spectool target but routes all storage through a
# real MinIO container to exercise the S3 backend end-to-end.
#
# Lifecycle:
#   1. Start MinIO Docker container (pinned image, --cidfile for clean teardown)
#   2. Wait for MinIO health endpoint
#   3. Create test bucket via aws CLI
#   4. Generate /tmp/flowy-s3.yaml with S3 storage config on port $(S3_PORT)
#   5. Start flowy-server and flowy-daemon with -config $(S3_CONFIG) and AWS credentials
#   6. Run spectool (same args as regular target)
#   7. Tear down daemon, server, MinIO regardless of spectool exit code

spectool-s3: $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
	@# Clean up stale data from previous runs.
	@docker run --rm -v $(abspath .)/flowy-data-s3:/data alpine sh -c "rm -rf /data/*" 2>/dev/null || true
	@rm -rf ./flowy-data-s3
	@mkdir -p ./flowy-data-s3
	@# Remove any leftover MinIO container from a previous interrupted run.
	@if [ -f $(MINIO_CID) ]; then docker rm -f $$(cat $(MINIO_CID)) 2>/dev/null || true; rm -f $(MINIO_CID); fi
	@# Start MinIO container in the background; --cidfile records the container ID.
	docker run --rm -d --cidfile $(MINIO_CID) \
		-e MINIO_ROOT_USER=minioadmin \
		-e MINIO_ROOT_PASSWORD=minioadmin \
		-p $(MINIO_PORT):9000 \
		$(MINIO_IMAGE) server /data
	@echo "Waiting for MinIO to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s -o /dev/null http://localhost:$(MINIO_PORT)/minio/health/live 2>/dev/null; then \
			echo "MinIO is ready."; \
			break; \
		fi; \
		if [ $$i -eq 10 ]; then \
			echo "ERROR: MinIO failed to start"; \
			docker rm -f $$(cat $(MINIO_CID)) 2>/dev/null; rm -f $(MINIO_CID); \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@# Create the test bucket.
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
		aws --endpoint-url http://localhost:$(MINIO_PORT) s3 mb s3://$(TEST_BUCKET) --region us-east-1
	@# Generate temporary S3 config on port $(S3_PORT) (avoids conflict with regular spectool on 8124).
	@printf 'base_dir: ./flowy-data-s3\nstorage:\n  type: s3\n  bucket: %s\n  endpoint: http://localhost:%s\n  region: us-east-1\nserver:\n  listen_addr: ":%s"\ndaemon:\n  server_url: "http://localhost:%s"\n  workers: 4\nclient:\n  server_url: "http://localhost:%s"\n' \
		$(TEST_BUCKET) $(MINIO_PORT) $(S3_PORT) $(S3_PORT) $(S3_PORT) > $(S3_CONFIG)
	@# Start flowy-server with S3 config and AWS credentials.
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
		$(SERVER_BIN) -config $(S3_CONFIG) &  echo $$! > $(SERVER_PID)
	@echo "Waiting for flowy-server (S3 mode) to be ready..."
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		if curl -s -o /dev/null http://localhost:$(S3_PORT)/runs 2>/dev/null; then \
			echo "flowy-server is ready."; \
			break; \
		fi; \
		if [ $$i -eq 10 ]; then \
			echo "ERROR: flowy-server (S3 mode) failed to start"; \
			kill $$(cat $(SERVER_PID)) 2>/dev/null; rm -f $(SERVER_PID); \
			docker rm -f $$(cat $(MINIO_CID)) 2>/dev/null; rm -f $(MINIO_CID); \
			rm -f $(S3_CONFIG); \
			exit 1; \
		fi; \
		sleep 1; \
	done
	@# Start flowy-daemon with S3 config and AWS credentials.
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin \
		$(DAEMON_BIN) -config $(S3_CONFIG) --workers 4 &  echo $$! > $(DAEMON_PID)
	@sleep 1
	@echo "flowy-daemon (S3 mode) started."
	@# Run spectool tests against S3 backend; capture exit code so we always clean up.
	$(eval EXCL := $(EXCLUDE_PATTERNS))
	$(SPECTOOL_BIN) test \
		"$(abspath $(FLOWY_BIN)) run -config $(S3_CONFIG) --input ~{input} ~{path}" \
		--redirect-stdout \
		--all-capabilities \
		--workflow-target-args="" \
		--task-target-args="--task ~{target}" \
		-s $(WDL_SPEC_DIR) \
		-c ./flowy-data-s3/spectool \
		$(if $(EXCL),--exclude "$(EXCL)",) \
	; SPECTOOL_EXIT=$$?; \
	echo "Stopping flowy-daemon (PID $$(cat $(DAEMON_PID)))..."; \
	kill $$(cat $(DAEMON_PID)) 2>/dev/null; rm -f $(DAEMON_PID); \
	echo "Stopping flowy-server (PID $$(cat $(SERVER_PID)))..."; \
	kill $$(cat $(SERVER_PID)) 2>/dev/null; rm -f $(SERVER_PID); \
	echo "Stopping MinIO container..."; \
	docker rm -f $$(cat $(MINIO_CID)) 2>/dev/null; rm -f $(MINIO_CID); \
	rm -f $(S3_CONFIG); \
	wait; \
	exit $$SPECTOOL_EXIT

# --- Cleanup ---

clean:
	rm -f $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN)
	rm -f $(SERVER_PID) $(DAEMON_PID)
