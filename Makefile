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

.PHONY: build spectool test clean

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

# --- Cleanup ---

clean:
	rm -f $(FLOWY_BIN) $(SERVER_BIN) $(DAEMON_BIN)
	rm -f $(SERVER_PID) $(DAEMON_PID)
