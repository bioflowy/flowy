---
phase: quick-8
plan: 1
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/server/dispatch.go
  - wdl/workflow/convert.go
  - wdl/task/inputs.go
autonomous: true
---

<objective>
Fix two S3 spectool failures:

1. **primitive_literals** -- S3 URI leaks into File/Directory workflow outputs because `jsonToValue` in dispatch.go treats all strings as `StringValue`, losing File/Directory type information. In S3 mode, task outputs contain `s3://bucket/.../hello.txt` strings that should be recognized as `FileValue` (or `DirectoryValue`) so the workflow evaluator preserves the correct type for spectool comparison.

2. **serde_homogeneous_pair** -- Go map iteration order is non-deterministic, causing `MapValue.Entries` to have random key ordering. All five sites where `map[string]interface{}` is iterated to build `MapValue.Entries` must sort keys before iteration.

Purpose: Fix remaining S3 spectool test failures
Output: Modified dispatch.go, convert.go, inputs.go with targeted fixes
</objective>

<context>
@internal/server/dispatch.go
@wdl/workflow/convert.go
@wdl/task/inputs.go
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix S3 URI leak in jsonToValue and add deterministic map key ordering in dispatch.go</name>
  <files>internal/server/dispatch.go</files>
  <action>
In `internal/server/dispatch.go`, make two changes:

**Change 1: S3 URI to FileValue detection**

In `jsonToValue`, modify the `case string:` branch to detect s3:// prefixed strings and return FileValue instead of StringValue. This preserves File type information so the workflow evaluator and spectool correctly handle File outputs from S3 mode.

```go
case string:
    if strings.HasPrefix(val, "s3://") {
        return values.FileValue{Type: types.FileType{}, Value: val}, nil
    }
    return values.StringValue{Type: types.StringType{}, Value: val}, nil
```

Add `"sort"` and `"strings"` to the import block.

**Change 2: Deterministic map key ordering**

In `jsonToValue`, in the `case map[string]interface{}:` branch (around line 191-206), replace the direct `for k, v := range val` iteration with sorted key iteration:

```go
case map[string]interface{}:
    keys := make([]string, 0, len(val))
    for k := range val {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    entries := make([]values.MapEntry, 0, len(val))
    for _, k := range keys {
        ev, err := jsonToValue(val[k])
        if err != nil {
            return nil, err
        }
        entries = append(entries, values.MapEntry{
            Key:   values.StringValue{Type: types.StringType{}, Value: k},
            Value: ev,
        })
    }
    return values.MapValue{
        Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
        Entries: entries,
    }, nil
```

Also apply the same sorted-key pattern in `rawToValues` (line 151): replace `for k, v := range raw` with sorted key iteration to ensure deterministic output key ordering:

```go
func rawToValues(raw map[string]interface{}, taskName string) (map[string]values.Value, error) {
    keys := make([]string, 0, len(raw))
    for k := range raw {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    result := make(map[string]values.Value, len(raw))
    for _, k := range keys {
        wv, err := jsonToValue(raw[k])
        if err != nil {
            return nil, fmt.Errorf("task %q output %q: %w", taskName, k, err)
        }
        result[k] = wv
    }
    return result, nil
}
```

Note: `rawToValues` returns `map[string]values.Value` which is itself unordered, but sorting here ensures consistent behavior if the map is iterated in the future. The critical sort is in `jsonToValue`'s MapValue construction.
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go build ./internal/server/...</automated>
  </verify>
  <done>dispatch.go compiles with s3:// FileValue detection and sorted map keys in both jsonToValue and rawToValues</done>
</task>

<task type="auto">
  <name>Task 2: Add deterministic map key ordering in convert.go and inputs.go</name>
  <files>wdl/workflow/convert.go, wdl/task/inputs.go</files>
  <action>
Apply sorted key iteration in all four remaining sites where `map[string]interface{}` is iterated to build `MapValue.Entries`.

**File 1: `wdl/workflow/convert.go`**

Add `"sort"` to import block (it is not currently imported).

Site 1 -- `jsonToValueTyped`, `case types.MapType:` (around line 202-210):
Replace `for k, mv := range m` with sorted key iteration:
```go
case types.MapType:
    if m, ok := v.(map[string]interface{}); ok {
        keys := make([]string, 0, len(m))
        for k := range m {
            keys = append(keys, k)
        }
        sort.Strings(keys)
        entries := make([]values.MapEntry, 0, len(m))
        for _, k := range keys {
            kv := jsonToValueTyped(k, dt.KeyType, structDefs, wdlDir)
            vv := jsonToValueTyped(m[k], dt.ValType, structDefs, wdlDir)
            entries = append(entries, values.MapEntry{Key: kv, Value: vv})
        }
        return values.MapValue{Type: dt, Entries: entries}
    }
    return jsonToValueUntyped(v)
```

Site 2 -- `jsonToValueUntyped`, `case map[string]interface{}:` (around line 280-288):
Replace `for k, mv := range x` with sorted key iteration:
```go
case map[string]interface{}:
    keys := make([]string, 0, len(x))
    for k := range x {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    entries := make([]values.MapEntry, 0, len(x))
    for _, k := range keys {
        entries = append(entries, values.MapEntry{
            Key:   values.StringValue{Type: types.StringType{}, Value: k},
            Value: jsonToValueUntyped(x[k]),
        })
    }
    return values.MapValue{
        Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
        Entries: entries,
    }
```

**File 2: `wdl/task/inputs.go`**

Add `"sort"` to import block (it is not currently imported).

Site 3 -- `jsonToWDLValue`, `case types.MapType:` (around line 171-188):
Replace `for k, mv := range m` with sorted key iteration:
```go
case types.MapType:
    m, ok := v.(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("expected object for Map[...], got %T", v)
    }
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    entries := make([]values.MapEntry, 0, len(m))
    for _, k := range keys {
        kv, err := jsonToWDLValue(k, dt.KeyType, structDefs, wdlDir)
        if err != nil {
            return nil, fmt.Errorf("map key %q: %w", k, err)
        }
        vv, err := jsonToWDLValue(m[k], dt.ValType, structDefs, wdlDir)
        if err != nil {
            return nil, fmt.Errorf("map value for key %q: %w", k, err)
        }
        entries = append(entries, values.MapEntry{Key: kv, Value: vv})
    }
    return values.MapValue{Type: dt, Entries: entries}, nil
```

Site 4 -- `jsonToValueUntyped`, `case map[string]interface{}:` (around line 262-270):
Replace `for k, val := range x` with sorted key iteration:
```go
case map[string]interface{}:
    keys := make([]string, 0, len(x))
    for k := range x {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    entries := make([]values.MapEntry, 0, len(x))
    for _, k := range keys {
        entries = append(entries, values.MapEntry{
            Key:   values.StringValue{Type: types.StringType{}, Value: k},
            Value: jsonToValueUntyped(x[k]),
        })
    }
    return values.MapValue{
        Type:    types.MapType{KeyType: types.StringType{}, ValType: types.AnyType{}},
        Entries: entries,
    }
```
  </action>
  <verify>
    <automated>cd /home/uehara/flowy && go build ./wdl/... && go test ./wdl/workflow/ ./wdl/task/ -count=1 -timeout=60s</automated>
  </verify>
  <done>All four map iteration sites use sorted keys. wdl packages compile and existing tests pass.</done>
</task>

</tasks>

<verification>
```bash
cd /home/uehara/flowy && go build ./... && go test ./internal/server/ ./wdl/workflow/ ./wdl/task/ ./wdl/values/ -count=1 -timeout=120s
```
</verification>

<success_criteria>
- `jsonToValue` in dispatch.go returns `FileValue` for `s3://` prefixed strings
- All 5 map iteration sites produce deterministic (sorted) key ordering for MapValue.Entries
- All packages compile and existing tests pass
</success_criteria>
