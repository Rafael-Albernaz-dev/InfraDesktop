# Regras de qualidade

Tudo o que barra código ruim no InfraDesktop mora aqui. As únicas exceções são as regras ESLint próprias, em `frontend/quality/`, porque precisam do `node_modules` do frontend.

- **Uma entrada:** `make quality` roda `quality/run.sh`, que executa todas as verificações e termina com uma tabela de id, status e tempo. O hook de pre-commit e o CI chamam o mesmo alvo.
- **Catraca:** dívida antiga fica congelada nas bases de `quality/baselines/` e em `frontend/quality/quality-ratchet.json`. Ela pode diminuir, nunca crescer. Código tocado sai dentro da regra.

## Pastas

| Pasta | Conteúdo |
|---|---|
| `go/` | `quality.go` (medição e regras Go); `qualityrules/` (catraca, baseline, limites, renomeação pelo Git); `qualitymetrics/` (medidor); `qualitydup/` (duplicação e sua base) |
| `architecture/` | Guard Go (`architecture.go`) e guard do frontend (`architecture-frontend.mjs`, com testes) |
| `layout/` | Veredito e guard das telas (`layout-verdict.mjs`, `layout-guard.mjs`, com testes) |
| `checks/` | Verificações avulsas: versão única e domínio de rádio no frontend |
| `baselines/` | Base Go (`quality-baseline.json`) e exceções de camada do frontend (`frontend-layer-exceptions.json`) |
| `../frontend/quality/` | Regras ESLint próprias, seus testes e a catraca do frontend |

## Catálogo

| Id | O que mede | Limite | Onde roda | Como corrigir |
|---|---|---|---|---|
| GO-FMT | Go formatado | `gofmt -l` vazio | `make quality` | `gofmt -w <arquivo>` |
| GO-VET | Erros comuns de Go | zero achado | `make quality` | corrigir o achado; nunca silenciar |
| GO-STATIC | Análise estática (código morto, uso errado de API) | zero achado | `make quality` | corrigir; código sem uso sai |
| FE-FORMAT | Frontend formatado | Prettier limpo | `make quality` | `npx prettier --write <arquivo>` |
| VERSION | Uma única fonte de versão | `VERSION` igual em pacote, Wails e npm | `make quality` | mudar só `VERSION` |
| GATES-TEST | Os próprios gates Go funcionam | testes verdes | `make quality` | corrigir o gate, não o teste |
| FE-RULES | As regras ESLint próprias funcionam | testes verdes | `make quality` | idem |
| QG1 | Arquitetura: domínio sem banco, SSH, Wails ou rede; frontend em camadas iguais às dos módulos Go (`access` < `equipment` < `inventory`/`monitoring` < `pppoe` < `radio`); só `app/` importa `app/`; `shared/` folha; bindings só em `platform/wails/` | zero violação; exceções em `baselines/frontend-layer-exceptions.json`, que só diminui | `make check` (`architecture-check`) | mover o arquivo para a camada certa, ou compor em `app/pages/` |
| QG2 | Tamanho | função ≤ 60 linhas; arquivo Go ≤ 500, `.tsx` ≤ 600, frontend ≤ 500; linha ≤ 140 | `make quality` (QG2-QG7, FE-LINT) | dividir por regra do domínio ou da tela, nunca em pedaços sem nome |
| QG3 | Complexidade | ≤ 15 por função tocada | idem | extrair a regra com nome; mapa no lugar de ternário aninhado |
| QG4 | Literal de domínio (papel, família, modo, código) fora do pacote dono | proibido | QG2-QG7 (Go), FE-LINT (`radio-domain`) | usar a constante do dono |
| QG5 | Nome de teste por comportamento, nunca por slice (`L07`, `BF04`, `Piloto`…) | proibido | QG2-QG7 | renomear pelo que o teste prova |
| G6 | Exceção legada (`legacyRatchetOnly` em `go/quality.go`) | não cresce | QG2-QG7 | tirar o arquivo da exceção quando ele entra na regra |
| G7 | Duplicação Go | janelas ≤ teto em `go/qualitydup/baseline.json` | QG2-QG7 | extrair helper nomeado no pacote dono |
| Catraca | Métrica de arquivo com dívida antiga | nunca piora; chave segue renomeação do Git | QG2-QG7 (Go), FE-LINT (frontend) | trazer o arquivo para dentro da regra |
| FE-LINT | ESLint do frontend: limites, hooks (`state-count` ≤ 8, `ref-count` ≤ 6), kit de UI, sem `eslint-disable` | zero aviso | `make quality` | seguir a mensagem; o kit fica em `src/shared/ui/` |
| QG9 | Telas fiéis ao wireframe congelado, só com o kit | guard e `make test-layout` | QG9 (`make quality`) e `make test-layout` (local, antes do push) | ver a skill de layout |
| LAYOUT-TEST | O veredito e o guard de layout funcionam | testes verdes | `make quality` | corrigir o gate |

## Regras de processo (fora do código)

| Id | Regra |
|---|---|
| QG6 | Arquivo em `scripts/` só se o `Makefile`, o CI ou um gate o chamar. Evidência descartável vai para `docs/evidence/` |
| QG7 | Lint, tipos e gates rodam de novo depois da última edição |
| QG8 | Revisão independente antes do build |
