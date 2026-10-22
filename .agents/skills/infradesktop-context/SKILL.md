---
name: infradesktop-context
description: Use when maintaining InfraDesktop project context.
version: 0.1.0
platforms: [linux]
---
# Contexto e curadoria InfraDesktop

## Quando usar

Atualizar documentação de estado/decisões/roadmap/lições ou procedimento recorrente. Não usar para reescrever fonte original, mudar produto por conveniência ou acumular diário.

## Procedimento

1. Ler AGENTS.md e as fontes locais docs/contexto.md/docs/Roadmap.md e decisão/evidência afetada com read_file. Se clone público não trouxer docs/, obter pacote local quando necessário; não inventar estado. Confirmar diferença e autoridade antes de editar.
2. Direcionar: regra → AGENTS.md; estado → docs/contexto.md; sequência → docs/Roadmap.md; decisão → docs/decisions; arquitetura → docs/architecture; preparo → docs/development; aprendizado → docs/memory.md; procedimento → .agents/skills; logs → .local. Todo docs/ é local/não versionado; documentação oficial futura é separada. Não duplicar autoridade.
3. Usar patch para alterações e write_file para novos arquivos. Preservar fontes importadas e distinguir proposto/aprovado/implementado/provado. Reconciliar frases atuais obsoletas em vez de empilhar overrides; histórico continua datado e ligado.
4. Atualizar procedimento só por repetição/pitfall comprovado. Preferir as três skills existentes a micro-skills por bug. Descrição curta, gatilho claro, passos verificáveis, pitfalls e verificação; paths relativos e sem segredo.
5. Antes de skill externa: demonstrar lacuna, consultar fonte oficial/upstream, licença, scripts/efeitos e pin de commit; não instalar biblioteca inteira nem mudar perfil global/trust por rotina. Skill não altera autorização ou ARQ-001.
6. Validar links, frontmatter/nomes/catálogo, estado/aceite e ausência de comandos alegados como executados sem prova. Documento-only não exige teste de produto inexistente.

## Armadilhas

memory.md não é memória global do Hermes nem fonte superior a decisão. AGENTS.md e HERMES.md não podem divergir: HERMES remete à fonte compartilhada. Não herdar ignore privado do Atlas: publicação deste projeto tem revisão própria. Skills locais recém-escritas não significam descoberta automática na sessão atual.

## Verificação

Cada fato tem fonte e lugar único; links funcionam; decisões fechadas não reaparecem como abertas; prompts ativos preservam escopo até instrução posterior. Relatar o que mudou/foi verificado e o próximo gate real, sem replay de processo.
