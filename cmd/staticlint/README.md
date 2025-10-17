# staticlint

Custom multichecker for this project.

## Usage

```bash
go run ./cmd/staticlint -- ./...
```

It aggregates:
- Standard analyzers from `golang.org/x/tools/go/analysis/passes`
- All `SA*` analyzers from `honnef.co/go/tools/staticcheck`
- Select analyzers from `simple` (S1000, S1009) and `stylecheck` (ST1000)
- Public analyzers: `bodyclose` and `exportloopref`
- Custom rule `noosexit`: forbids direct `os.Exit` calls in `main.main`

## CI

```bash
go run ./cmd/staticlint -- ./...
```
