SHELL := /bin/bash
export PATH := $(CURDIR)/.tools/go1.27.3/go/bin:$(CURDIR)/.tools/bin:$(PATH)
export GOTOOLCHAIN := local
export GOPROXY := off
export GOSUMDB := off
export GOFLAGS := -mod=readonly

.PHONY: tools-check bindings assets gofmt-check architecture-check check test-integration build package test-desktop run test-visual test-scroll icon-check test-dev

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
check: tools-check assets gofmt-check architecture-check test-visual-integrity
	go vet -tags webkit2_41 ./...
	go test -tags webkit2_41 ./...
	go test -race -tags webkit2_41 ./...
	staticcheck -tags webkit2_41 ./...
	npm --prefix frontend run format:check
	npm --prefix frontend run lint
	npm --prefix frontend run typecheck
	npm --prefix frontend run test -- --run
	npm --prefix frontend run build
test-integration: assets
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -o build/bin/infradesktopd ./cmd/infradesktopd
	go test -count=1 -v -tags webkit2_41 ./internal/adapters/ipc -run TestIntegration
	python3 scripts/test-process.py
build: tools-check assets
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -o build/bin/infradesktopd ./cmd/infradesktopd
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

test-visual: assets test-visual-integrity
	node scripts/test-visual.mjs
	$(MAKE) test-scroll

test-scroll:
	node scripts/test-scroll.mjs "$$(cat build/ui-021-visual/latest.txt)"

icon-check:
	python3 scripts/generate-infradesktop-icon.py --check

test-dev:
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/test-dev.py
