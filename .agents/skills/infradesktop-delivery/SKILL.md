---
name: infradesktop-delivery
description: Use when scoping an InfraDesktop Codex delivery.
version: 0.1.0
platforms: [linux]
---
# Entrega InfraDesktop

## Quando usar

Preparar ou corrigir recorte e passagem para Codex. Não usar para implementar produto pelo Hermes nem para publicar Git automaticamente.

## Procedimento

1. Ler AGENTS.md e, no ambiente local preparado, docs/contexto.md/docs/Roadmap.md com read_file; localizar decisões/fontes por search_files. docs/ é ignorado e não existe no clone público: solicitar fontes locais necessárias, sem inventar requisitos. Confirmar escopo atual; decisões respondidas não viram novo grill.
2. Conferir destino/Git se existir, ferramentas, scripts e ownership com terminal; dependências ausentes não autorizam instalação. Critério: cada efeito necessário tem autorização ou gate explícito.
3. Definir objetivo, entradas/saídas, limite, falhas e oráculo. Separar mecanismo sintético de capacidade real; não criar módulos/integrações vazios. Perguntar em texto apenas decisões materiais não recuperáveis dos arquivos; agrupar até cinco se houver volume.
4. Escrever despacho local com write_file em docs/prompts, após recorte autorizado; nunca versionar docs/. Estratégia antes do corpo: etapa, motivo, estimativa aproximada, modelo disponível, CLI, agentes e worktree por risco real. Incluir destino/branch real, fontes, contratos, efeitos, não objetivos, ARQ-001 integral de AGENTS.md, provas e Do not commit se commit não autorizado.
5. Incluir implementação → prova focal → autocorreção → fechamento no mesmo escopo, sem nova pergunta para defeito rotineiro. Solicitar nova autoridade só por decisão/efeito material; interromper somente parte afetada.
6. Exigir evidências/exits e walkthrough numerado com comandos reais esperados, dados sintéticos, falhas, limitações e cleanup próprio. Prompt existente é referência, não autorização de execução.

## Armadilhas

Sem Git/manifest/script, não inventar branch/comando existente. Prompt preparado não prova CI/build nem Wails. L01 não requer equipamento/LLM/cofre completo; boot/instalação mínima não podem ser dispensados no fechamento. Não copiar limites/branches/política de Atlas.

## Verificação

Cada aceite tem prova e cada efeito tem escopo; guardrail integral presente; nenhum teste de outra capacidade imposto por rotina; pendências não convertidas em PASS. Atualizar contexto/roadmap quando estado mudar, mantendo fonte única.
