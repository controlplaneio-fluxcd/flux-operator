---
name: go-check
description: Run go vet to analyze Go source code for suspicious constructs
---

# Go Check

Use `go vet` to examine Go source code and report suspicious constructs
that the compiler might not catch.

## Usage

To vet the current package:

```bash
go vet ./...
```

To vet a specific package:

```bash
go vet ./pkg/mypackage
```

## Common Options

- `-v` — verbose output, listing checked files
- `-json` — output diagnostics in JSON format
- `-vettool=path` — use an external vet tool

## Analyzers

`go vet` runs several built-in analyzers including:

- `printf` — check printf-style format strings
- `shadow` — check for shadowed variables
- `structtag` — check struct field tags conform to `reflect.StructTag.Get`
- `unusedresult` — check for unused results of calls to certain functions
