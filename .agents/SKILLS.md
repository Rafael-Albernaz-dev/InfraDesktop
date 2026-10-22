# Skills locais

Procedimentos versionáveis, específicos do InfraDesktop; fonte de regras é AGENTS.md, estado local em docs/contexto.md. Todo docs/ é ignorado; um clone público não o contém. Não são instalação global Hermes nem plugin. Solicitar fontes locais quando necessárias, sem transformar docs/ em dependência da CI pública.

- skills/infradesktop-delivery/SKILL.md: recortar capacidade e produzir despacho completo com ARQ-001 e provas proporcionais.
- skills/infradesktop-audit/SKILL.md: auditar retorno/arquitetura/ciclo de vida, distinguir alegação de evidência.
- skills/infradesktop-context/SKILL.md: manter estado/decisões/roadmap/lições e revisar skills sem duplicação.

Carregar somente a pertinente. Hermes suporta .agents/skills em projetos Git confiados segundo documentação oficial; até init/trust/nova sessão, usar leitura explícita por caminho. Codex deve seguir AGENTS.md e carregar a skill necessária, sem pressupor catálogo da sessão atualizado.

Nenhuma skill externa instalada. Avaliar nova dependência de procedimento só por lacuna concreta; revisar origem/licença/scripts/pin e precedência antes de adotar.
