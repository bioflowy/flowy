# Verification 41

## Automated Checks

- `GOCACHE=/tmp/flowy-go-cache go test ./internal/server ./internal/store/sqlite`

## Covered Behavior

- Valid label templates produce human-readable managed-file labels.
- Missing variables and malformed templates fall back to basename labels.
- Existing completion and managed-file registration tests remain green.
