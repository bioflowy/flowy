FLOWY_BIN     := ./flowy
SPECTOOL_BIN  := $(HOME)/.cargo/bin/spectool
WDL_SPEC_DIR  := /tmp/wdl-spec
CARGO         := $(HOME)/.cargo/bin/cargo

# Comma-separated list of test name substrings to skip.
# Loaded from spectool-exclude.txt (strips comments and blank lines).
EXCLUDE_PATTERNS := $(shell grep -v '^\s*\#' spectool-exclude.txt 2>/dev/null | grep -v '^\s*$$' | paste -sd ',' -)

.PHONY: build spectool test clean

# --- Build targets ---

build: $(FLOWY_BIN)

$(FLOWY_BIN): $(shell find cmd wdl -name '*.go' 2>/dev/null)
	go build -o $(FLOWY_BIN) ./cmd/flowy

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

spectool: $(FLOWY_BIN) $(SPECTOOL_BIN) $(WDL_SPEC_DIR)
	$(eval EXCL := $(EXCLUDE_PATTERNS))
	$(SPECTOOL_BIN) test \
		"$(abspath $(FLOWY_BIN)) run --input ~{input} ~{path}" \
		--redirect-stdout \
		--all-capabilities \
		--workflow-target-args="" \
		--task-target-args="--task ~{target}" \
		-s $(WDL_SPEC_DIR) \
		$(if $(EXCL),--exclude "$(EXCL)",)

# --- Cleanup ---

clean:
	rm -f $(FLOWY_BIN)
