# 3270Connect – Agent Onboarding Notes

## What this repo is
- Go application that automates 3270 mainframe workflows; provides CLI, API server, and optional dashboard UI (HTML templates in `templates/` and static assets in `app/static` / `site`).
- Ships embedded x3270/s3270 binaries (in `binaries/`) and sample apps (`sampleapps/app1`, `sampleapps/app2`).
- Repo includes prebuilt binaries under `dist/` (Linux/Windows) plus website sources in `docs/` (MkDocs). Primary entrypoint: `go3270Connect.go` (root); supporting TN3270 client logic in `connect3270/`.
- Tooling/language: Go (module `github.com/3270io/3270Connect`, go 1.21 with toolchain go1.23.1). No custom linter; use `gofmt`/`go vet` as needed.

## Repo layout (quick map)
- Root: `go3270Connect.go` (main), `connect3270/` (emulator wrapper), `templates/` (dashboard HTML), `binaries/` (embedded emulator executables; regenerated via `update-binaries.ps1` → `go-bindata`), `dist/` (build outputs), `docs/` & `mkdocs.yml` (site), `build.sh` / `build.ps1` (cross-build helpers), `Dockerfile*`, `workflow*.json` (sample workflow configs), `exampleInputFile.txt`, `Makefile` (x3270 upstream builder), `BUILD.md` (legacy Windows notes).
- CI: `.github/workflows/go.yml` runs on PR/main: Windows job sets up Go 1.23, runs `./build.ps1`, then `go test -v ./...`; Ubuntu jobs (commented docker build) and MkDocs build/deploy to Pages.

## Prereqs & environment
- Go 1.23.x recommended (matches CI/toolchain). CGO disabled in shipped builds. `rsrc` is needed for Windows icon embedding; both build scripts auto-install it if missing.
- Building docs requires Python 3 and `mkdocs-material`, `pymdown-extensions`, `mkdocs-video`, `mkdocs-simple-plugin` (see workflow).
- No database/external services required; x3270 binaries are bundled.

## Validated commands (run from repo root)
- `go test ./...` (success on Linux; fast, fetches modules on first run).
- `./build.sh` (success; installs `rsrc` if absent, emits `dist/3270Connect.exe` and `dist/3270Connect_linux` with `CGO_ENABLED=0` for both OS targets).
  - Preconditions: Go toolchain available, network to download `rsrc` and modules. Creates/updates `resource.syso` and `dist/`.
- Expected CI sequence (mirrors what to run before PR):
  1) `./build.sh` (or on Windows: `pwsh ./build.ps1`), 2) `go test -v ./...`.
- Docs site: `pip install --upgrade mkdocs-material pymdown-extensions mkdocs-video mkdocs-simple-plugin` then `mkdocs build` (per workflow); publishes into `site/`.

## Common workflows
- Build binaries manually (Linux host): `./build.sh` (does Windows+Linux builds; uses `GOOS/GOARCH` switches). For Windows-only builds with icon embedding: `pwsh ./build.ps1`.
- Update embedded emulator binaries: refresh files under `binaries/linux` or `binaries/windows`, then run `./update-binaries.ps1` (runs `go-bindata` to regenerate `binaries/bindata.go`).
- Run CLI: `./dist/3270Connect_linux -headless -config workflow.json` (or the Windows exe). Sample input files: `workflow.json`, `workflow_inputfile.json`, `exampleInputFile.txt`.
- Dashboard templates live in `templates/`; static assets in `app/static`/`site`. API/server paths are defined in `go3270Connect.go`.

## Tips & gotchas
- Go module sets `toolchain go1.23.1`; older Go may fail. Keep `CGO_ENABLED=0` for the provided build scripts (matches CI and avoids system libs).
- `build.sh`/`build.ps1` overwrite/create `resource.syso` and drop artifacts in `dist/`; clean manually if needed.
- CI uses Windows runner for build/test—if you change build flags, ensure Windows compatibility.
- No dedicated lint step in CI; running `gofmt ./...` and optionally `go vet ./...` locally is safe.
- MkDocs build assumes `docs/` as source and writes to `site/`; Pages workflow adds `site/CNAME`.
- x3270 upstream builder `Makefile` is optional; only needed when rebuilding emulator binaries (downloads upstream repo to `/tmp/x3270-build`).

## Where to look when editing
- Core logic: `go3270Connect.go` (CLI, API, dashboard), `connect3270/emulator.go` (TN3270 process control/retries), `charmui.go`/`open_dashboard_*.go` (UI helpers).
- Templates: `templates/*.gohtml`; static assets in `app/static/` (built site copy) and `templates/static`.
- Samples: `sampleapps/` for demo apps and workflow examples; `docs/*.md` for product documentation.

## Working efficiently
- Follow the validated command order: build (or build+rsrc) → `go test ./...`. Re-run after changes.
- Trust these notes first; search the tree only if something here is missing or incorrect.
