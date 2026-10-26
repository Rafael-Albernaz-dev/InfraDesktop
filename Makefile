SHELL := /bin/bash
export PATH := $(CURDIR)/.tools/go1.27.3/go/bin:$(CURDIR)/.tools/bin:$(PATH)
export GOTOOLCHAIN := local
export GOPROXY := off
export GOSUMDB := off
export GOFLAGS := -mod=readonly

.PHONY: part1-harness test-part1-desktop test-part1-visual test-vault-helper test-part1-production test-vault-read-race test-baseline-tooling test-equipment-smoke
.PHONY: daemon-build tools-check bindings assets gofmt-check architecture-check check test-integration build package test-desktop run test-visual test-scroll icon-check test-dev

tools-check:
	bash scripts/tools-check.sh
bindings:
	bash scripts/bindings.sh
assets: bindings
	npm --prefix frontend run build
gofmt-check:
	@test -z "$$(gofmt -l main.go internal cmd scripts/*.go)" || (gofmt -l main.go internal cmd scripts/*.go; exit 1)
architecture-check: assets
	go run ./scripts/architecture.go
	node --test scripts/architecture-frontend.test.mjs
	node scripts/architecture-frontend.mjs
check: tools-check assets gofmt-check architecture-check test-visual-integrity test-baseline-tooling
	go vet -tags webkit2_41 ./...
	go test -tags webkit2_41 ./...
	go test -race -tags webkit2_41 ./...
	staticcheck -tags webkit2_41 ./...
	npm --prefix frontend run format:check
	npm --prefix frontend run lint
	# assets already runs build = tsc --noEmit + Vite; no source mutates below it.
	npm --prefix frontend run test -- --run
test-integration: daemon-build
	go test -count=1 -v -tags webkit2_41 ./internal/adapters/ipc -run TestIntegration
	python3 scripts/test-process.py
daemon-build: assets
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -o build/bin/infradesktopd ./cmd/infradesktopd

build: tools-check daemon-build
	wails build -tags webkit2_41 -s -skipbindings -m -nosyncgomod -trimpath -o infradesktop
package: build
	python3 scripts/package.py

test-desktop:
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/test-desktop.py
	python3 scripts/test-window-fit.py

run: build
	python3 scripts/run-dev.py

test-visual-integrity:
	node --test scripts/visual-checks.test.mjs scripts/visual-actions.test.mjs
	python3 scripts/test-visual-integrity.py
	python3 scripts/test-native-visual-metadata.py

test-visual: assets test-visual-integrity
	node scripts/test-visual.mjs
	$(MAKE) test-scroll

test-scroll:
	node scripts/test-scroll.mjs "$$(cat build/ui-021-visual/latest.txt)"

icon-check:
	python3 scripts/generate-infradesktop-icon.py --check

test-dev:
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/test-dev.py

# Separate uninstalled daemon: explicit test key provider cannot be a fallback in
# the production daemon produced by `build` and packaged by `package`.
part1-harness:
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -ldflags '-X infradesktop/internal/composition.devVaultEnabled=true' -o build/bin/infradesktopd-part1-test ./cmd/infradesktopd
	go build -trimpath -o build/bin/part1-ssh-fixture ./scripts/part1-fixture

test-part1-desktop: build part1-harness
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/test-part1-desktop.py

test-part1-visual:
	node scripts/test-part1-visual.mjs

test-vault-helper:
	python3 scripts/test-vault-provision.py

test-part1-production: build
	python3 scripts/test-part1-production.py

test-vault-read-race:
	python3 scripts/test-vault-read-race.py

test-baseline-tooling:
	python3 scripts/test-baseline-tooling.py

test-equipment-smoke:
	node scripts/test-part1-visual.mjs --smoke
