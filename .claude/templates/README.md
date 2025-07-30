# Claude Templates

This directory contains Go template files that use placeholder syntax (e.g., `{Resource}`, `{Method}`) for generating new code.

## Important: CI/CD Configuration

These template files are **not valid Go syntax** and must be excluded from:
- `make fmt` - Handled in Makefile by excluding `.claude/*` path
- `make lint` - Handled in `.golangci.yml` by excluding `.claude` directory
- Git operations - `.gitignore` includes `.claude/templates/*.go`

## Why Templates Are Excluded

The templates use placeholder syntax like:
```go
func (api *{Resource}API) Handle{Resource}(c *gin.Context) {
```

This is invalid Go syntax and will cause:
- `gofmt` to fail with syntax errors
- `golangci-lint` to fail with parse errors
- Any Go tooling to report errors

## Usage

To use a template:
1. Copy the template file to your target location
2. Replace all placeholders (text in `{braces}`) with actual values
3. Run `make fmt` on the resulting file

## CI/CD Notes

The following configurations ensure templates don't break CI:

1. **Makefile** - `fmt` target uses `find` with `-not -path "./.claude/*"`
2. **.golangci.yml** - Configured with `skip-dirs: [.claude]`
3. **.gitignore** - Includes `.claude/templates/*.go` pattern

This ensures that CI pipelines will not fail due to template syntax.