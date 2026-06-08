# Swoop build helpers (thin wrappers over scripts/build.sh).
#
# The build script checks dependencies, installs only what is missing, updates
# only what is outdated, builds, and prints the binary location. On Linux it
# auto-detects WebKit2GTK 4.1 and uses the webkit2_41 build tag.

.PHONY: build check clean-deps dev doctor core-test cross-core

# Full flow: check + install/update dependencies, then build.
build:
	bash scripts/build.sh

# Check and install/update dependencies only (no build).
check:
	bash scripts/build.sh --check-only

# Remove exactly the dependencies the build script installed.
clean-deps:
	bash scripts/build.sh --clean

# Live development with hot reload (adds webkit2_41 tag on Linux when present).
dev:
	@tags=""; \
	if [ "$$(uname -s)" = "Linux" ] && pkg-config --exists webkit2gtk-4.1 2>/dev/null; then tags="-tags webkit2_41"; fi; \
	wails dev $$tags

doctor:
	wails doctor

# Compile and vet the platform-agnostic core (no CGO, runs anywhere).
core-test:
	go build ./core/...
	go vet ./core/...

# Sanity-check that the core cross-compiles for Linux and macOS (incl. Apple Silicon).
cross-core:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./core/...
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./core/...
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./core/...
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ./core/...
