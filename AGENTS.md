# AGENTS Notes

## Session memory rules
- Persist valuable decisions for future sessions; keep this file up to date.
- If the user asks to persist guidance, write it to `AGENTS.md` or another repo `.md` file that is easy to find later.
- Prefer minimal edits that achieve the goal; avoid broad rewrites when a small change is enough.
- Commit between units of work.

## Repository shape
- This repo is a single-binary Go CLI (entrypoint: `main.go`), not a multi-package app.
- Purpose: Helm post-renderer that replaces `Ingress` manifests with Gateway API manifests produced by `ingress2gateway`.
- `samples/sample/` is a Helm chart fixture for manual testing.

## Verified commands
- Build artifact path is defined by `Makefile`: `make build` -> `go build -o ./dist/ingress-modernizr main.go`.
- Direct build equivalent: `go build -o ./dist/ingress-modernizr main.go`.
- Focused manual check:
  1) `helm template <release> <chart> > before.yaml`
  2) `cat before.yaml | ./dist/ingress-modernizr --providers=ingress-nginx > after.yaml`
- Current repo has no `*_test.go`; `go test ./...` is expected to report no test files.

## Runtime behavior to preserve
- Input is read from `stdin` as a full multi-doc YAML/JSON stream.
- The tool writes all input manifests to a temp file, then runs `ingress2gateway print --input-file <temp>`.
- All CLI args are passed through to `ingress2gateway` (except a leading `print`, which is stripped).
- Output removes all original `kind: Ingress`, keeps other originals, then appends converted objects.
- `INGRESS2GATEWAY_BIN` overrides the `ingress2gateway` binary path (default: `ingress2gateway` on `PATH`).

## Conventions for future work
- For new CLI structure, use `spf13/cobra`.
- For new config handling, use `spf13/viper`.
- Use `ROADMAP.md` for planned work and remove items as they are completed.
