#!/usr/bin/env bash
# Entrada única das regras de qualidade (make quality e hook de pre-commit).
# Roda todas as verificações, mostra a saída só do que falhou e termina com uma tabela.
# Catálogo de cada id: quality/README.md.
set -u
cd "$(git rev-parse --show-toplevel)"

ids=() descs=() results=() times=()
logdir=$(mktemp -d)
trap 'rm -rf "$logdir"' EXIT

step() {
  local id=$1 desc=$2
  shift 2
  local start=$SECONDS
  if "$@" >"$logdir/$id.log" 2>&1; then
    results+=(ok)
  else
    results+=(FALHOU)
    echo "── $id · $desc ─────────────"
    tail -n 40 "$logdir/$id.log"
    echo
  fi
  ids+=("$id") descs+=("$desc") times+=($((SECONDS - start)))
}

gofmt_clean() {
  local out
  out=$(gofmt -l main.go internal cmd quality) || return 1
  [ -z "$out" ] || { echo "$out"; return 1; }
}

step GO-FMT      "Go formatado (gofmt)"                       gofmt_clean
step GO-VET      "go vet"                                     go vet -tags webkit2_41 ./...
step GO-STATIC   "staticcheck"                                staticcheck -tags webkit2_41 ./...
step FE-FORMAT   "frontend formatado (Prettier)"              npm --prefix frontend run format:check
step VERSION     "versão única (VERSION)"                     python3 quality/checks/test-version.py
step GATES-TEST  "testes dos próprios gates Go"               go test ./quality/go/qualityrules ./quality/go/qualitydup -count=1
step BE-LAYOUT   "estrutura do backend e testes do guard"     bash -c 'node quality/architecture/backend-layout.mjs && node --test quality/architecture/backend-layout.test.mjs'
step FE-RULES    "testes das regras ESLint próprias"          bash -c 'cd frontend && node --test quality/*.check.mjs'
step QG2-QG7     "tamanho, tipos, nomes, legado, duplicação"  go run ./quality/go/quality.go
step FE-LINT     "ESLint com catraca e kit (QG2–QG4, QG9)"    bash -c 'cd frontend && ./node_modules/.bin/eslint . --max-warnings 0 --no-inline-config'
step LAYOUT-TEST "testes do veredito e do guard de layout"    node --test quality/layout/layout-verdict.test.mjs quality/layout/layout-guard.test.mjs
step QG9         "guard de layout"                            node quality/layout/layout-guard.mjs

failed=0
# A descrição vai por último: tem acento, e printf alinha por byte.
printf '\n%-12s %-7s %-6s %s\n' ID STATUS TEMPO REGRA
for i in "${!ids[@]}"; do
  printf '%-12s %-7s %-6s %s\n' "${ids[$i]}" "${results[$i]}" "${times[$i]}s" "${descs[$i]}"
  [ "${results[$i]}" = ok ] || failed=1
done
if [ "$failed" = 1 ]; then
  echo; echo "quality: FALHOU. O que cada id mede e como corrigir: quality/README.md"
  exit 1
fi
echo; echo "quality: PASS"
