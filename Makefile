SHELL := /bin/bash
export PATH := $(CURDIR)/build/tools:$(CURDIR)/.tools/go1.27.3/go/bin:$(CURDIR)/.tools/bin:$(PATH)
export GOTOOLCHAIN := local
export GOPROXY := off
export GOSUMDB := off
export GOFLAGS := -mod=readonly

.PHONY: part1-harness test-part1-desktop test-part1-visual test-vault-helper test-part1-production test-vault-read-race test-baseline-tooling test-equipment-smoke
.PHONY: daemon-build tools-check bindings assets gofmt-check architecture-check check check-front check-go check-race check-race-shard ci-package test-integration build package test-desktop run test-visual test-scroll icon-check test-dev

tools-check:
	bash scripts/build/tools-check.sh
bindings:
	bash scripts/build/bindings.sh
assets: bindings
	npm --prefix frontend run build
gofmt-check:
	@test -z "$$(gofmt -l main.go internal cmd quality)" || (gofmt -l main.go internal cmd quality; exit 1)
architecture-check: assets
	go run ./quality/architecture/architecture.go
	node --test quality/architecture/architecture-frontend.test.mjs
	node quality/architecture/architecture-frontend.mjs
# check = check-front + check-go + check-race. O CI roda as três partes em jobs
# paralelos (e check-race em fatias, ver scripts/ci/race-shard.sh); localmente,
# `make check` continua rodando tudo.
check: check-front check-go check-race
check-front: tools-check assets gofmt-check architecture-check test-visual-integrity test-baseline-tooling
	staticcheck -tags webkit2_41 ./...
	npm --prefix frontend run format:check
	npm --prefix frontend run lint
	# assets already runs build = tsc --noEmit + Vite; no source mutates below it.
	npm --prefix frontend run test -- --run
check-go: tools-check assets
	go vet -tags webkit2_41 ./...
	go test -tags webkit2_41 ./...
# orçamento de tempo é medido no go test normal; sob -race só se procura corrida
RACE_SKIP := ^(TestIPCListsAndBulkUpdatesFiveHundredDevices|TestImportIPCPreviewAndConfirmFiveHundred|TestImportPersistsFiveHundredWithinBudget|TestL06T2BasePublicationReclaimsMaterialSeriesWithinBudget)$$
RACE_PKGS ?= ./...
RACE_RUN ?=
check-race: assets
	go test -race -tags webkit2_41 -skip '$(RACE_SKIP)' $(if $(RACE_RUN),-run '$(RACE_RUN)') $(RACE_PKGS)
# fatia de -race sem frontend nem Wails: só pacotes que não embutem frontend/dist
check-race-shard:
	go test -race -tags webkit2_41 -skip '$(RACE_SKIP)' $(if $(RACE_RUN),-run '$(RACE_RUN)') $(RACE_PKGS)
test-integration: daemon-build
	go test -count=1 -v -tags webkit2_41 ./internal/adapters/ipc -run TestIntegration
	python3 scripts/desktop/test-process.py
daemon-build: assets
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -o build/bin/infradesktopd ./cmd/infradesktopd

build: tools-check daemon-build
	wails build -tags webkit2_41 -s -skipbindings -m -nosyncgomod -trimpath -o infradesktop
package: build
	python3 scripts/build/package.py

test-desktop:
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/desktop/test-desktop.py
	python3 scripts/desktop/test-window-fit.py

run: build
	python3 scripts/dev/run-dev.py

test-visual-integrity:
	node --test scripts/visual/visual-checks.test.mjs scripts/visual/visual-actions.test.mjs
	python3 scripts/visual/test-visual-integrity.py
	python3 scripts/desktop/test-native-visual-metadata.py

test-visual: test-layout

test-scroll:
	node scripts/visual/test-scroll.mjs

icon-check:
	python3 scripts/build/generate-infradesktop-icon.py --check

test-dev:
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/dev/test-dev.py

# Separate uninstalled daemon: explicit test key provider cannot be a fallback in
# the production daemon produced by `build` and packaged by `package`.
part1-harness:
	mkdir -p build/bin
	go build -trimpath -tags webkit2_41 -ldflags '-X infradesktop/internal/composition.devVaultEnabled=true' -o build/bin/infradesktopd-part1-test ./cmd/infradesktopd
	go build -trimpath -o build/bin/part1-ssh-fixture ./scripts/desktop/part1-fixture

test-part1-desktop: build part1-harness
	xvfb-run -a -s "-screen 0 2000x1700x24 -nolisten tcp" python3 scripts/desktop/test-part1-desktop.py

test-part1-visual:
	node scripts/visual/test-part1-visual.mjs

test-vault-helper:
	python3 scripts/desktop/test-vault-provision.py

test-part1-production: build
	python3 scripts/desktop/test-part1-production.py

test-vault-read-race:
	python3 scripts/desktop/test-vault-read-race.py

test-baseline-tooling:
	python3 scripts/visual/test-baseline-tooling.py

test-equipment-smoke:
	node scripts/visual/test-part1-visual.mjs --smoke

.PHONY: quality ci
ci: tools-check assets quality check ci-package
# parte de empacotamento e processo do ci, rodada em job próprio no CI
ci-package: tools-check test-integration package test-vault-helper test-part1-production test-vault-read-race

quality:
	@test -d frontend/dist || (echo "rode make assets"; exit 1)
	bash quality/run.sh

.PHONY: test-layout
test-layout:
	node scripts/visual/test-layout.mjs

.PHONY: quality-report
quality-report:
	go run ./quality/go/quality.go -report
	cd frontend && node quality/eslint-quality-report.mjs
