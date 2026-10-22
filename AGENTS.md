# InfraDesktop — instruções compartilhadas

## Entrada e autoridade

Ler este arquivo. No ambiente local preparado, continuar por docs/contexto.md → docs/Roadmap.md, consultando apenas decisões/skills/fontes pertinentes. docs/memory.md contém aprendizados, não autorização. O índice local é docs/README.md. Toda a pasta docs/ é privada, ignorada e ausente no clone público: se uma execução depender dessas fontes, solicitar o pacote local, sem inventar requisitos. Não carregar todo o DRS em cada turno; leitura integral obrigatória na preparação inicial de L01.

Instrução atual explícita do usuário e exceções formalmente autorizadas → ARQ-001 e decisões aprovadas → DRS vigente → contratos/ADRs da capacidade → evidências → referências históricas. Estado real do filesystem/Git/código prevalece sobre alegações de implementação, mas código não revoga requisito aprovado. D61–D62 substituem o bloqueio histórico D59. Base aceita não significa capacidade homologada.

## Papéis e permissões

- Hermes: planejamento, grill focal, documentação, auditoria, contratos, skills e preparo local autorizado. Não implementa produto/testes em lugar do Codex por rotina.
- Codex CLI: executor principal do produto e testes em slice autorizada; corrige falhas dentro do mesmo escopo.
- Subagentes: leitura/inventário/revisão por padrão; sem efeitos externos implícitos.
- Escopo autorizado atual: importar/consolidar documentos, validar links, instalar dependências de preparo, inicializar Git, configurar CI documental, criar primeiros commits e push para Rafael-Albernaz-dev/InfraDesktop. Não inclui implementação do produto, conexão a equipamentos ou ativação do daemon no host. Instalação/boot em Ubuntu descartável requer alvo definido; senha sudo é entrada humana local, nunca solicitada no chat.
- Criar arquivos não autoriza instalar, habilitar serviços, conectar equipamentos, commitar ou publicar. Usar o escopo da instrução atual, sem repetir autorização genérica já resolvida.

## ARQ-001 — fonte canônica operacional

Clean Architecture pragmática, módulos por capacidade. Domínio/casos de uso independentes de Wails, banco, systemd, protocolos e LLM. UI apresenta; serviço local concentra autorização, conexões, coleta, análise, trabalhos e execução; adapters traduzem tecnologias e dependem para dentro. Composição explícita fora do núcleo. Sem interfaces/camadas ornamentais, módulos vazios ou bindings genéricos de filesystem/shell/SQL. Diretórios e lint sozinhos não comprovam arquitetura.

Exceção: antes de romper fronteira, apresentar motivo, impactos, alternativas e escopo; obter autorização explícita prévia e registrar decisão. Suspender somente parte afetada. Todo prompt do projeto inclui o bloco integral ARQ-001 abaixo; a cópia local em docs/governance/guardrail-prompts.md não é dependência do clone público.

> InfraDesktop — guardrail obrigatório ARQ-001: seguir Clean Architecture pragmática, com módulos por capacidade e domínio/casos de uso independentes de Wails, banco, protocolos e LLM. A interface apresenta informações; o serviço local concentra conexões, coleta, análises, trabalhos e execução; adapters cuidam das tecnologias externas, com dependências voltadas para dentro. Não criar camadas ou interfaces ornamentais. Não romper essas fronteiras sem motivo identificado, impactos e alternativas apresentados, autorização explícita prévia do usuário e decisão registrada com escopo. Se surgir essa necessidade, interromper somente a parte afetada e solicitar a decisão. Verificar a conformidade e apresentar a evidência adequada ao recorte.

## Trabalho e evidência

Antes de escrever: conferir destino/instruções, Git se existir, alterações staged/unstaged/untracked e scripts reais; preservar mudanças de terceiros. Não importar permissões/branches/orçamentos do Atlas, Hublink ou VigiaDev. Referências são somente leitura, sem copiar login/backend/regras particulares.

Testes pelo risco, oráculo explícito, regressão útil e gates que falham se ferramenta obrigatória faltar. Proibido esconder falha com skip, mock da própria unidade, assert enfraquecido ou timeout arbitrário. Reutilizar evidência válida; não repetir suítes amplas por rotina. Documentação exige links/consistência, não build de produto inexistente.

Distinguir observado, inferido, proposto, sintético e homologado. Build não prova Wails real; jsdom/browser não prova WebKitGTK; processo separado não prova boot. Instalação mínima/boot integram fechamento L01, cofre completo L02, visual/medição inicial L03. Sem prova, manter gate pendente.

Não ler/imprimir/commitar segredos, .env, inventário privado, dumps, credenciais ou dados reais de clientes. Testes padrão isolados e sintéticos. Cleanup somente de recursos próprios; nunca matar por nome/porta, reiniciar serviço alheio ou alterar rede por conveniência.

## Documentação e skills

Fonte única: AGENTS.md regras; docs/contexto.md estado local; docs/Roadmap.md sequência local; docs/decisions/ decisões; docs/architecture/ fronteiras; docs/development/ preparo; docs/memory.md lições; .agents/skills/ procedimentos. Sem cópia concorrente agents.md. Não reproduzir camadas de código em pastas documentais ornamentais. A documentação oficial de produto será definida separadamente, não promovendo o docs/ de preparação.

Skills locais: infradesktop-delivery (recorte/despacho), infradesktop-audit (evidência/ARQ-001), infradesktop-context (higiene/curadoria). Ler o SKILL.md pertinente. Não transferir skills inteiras de outro produto nem instalar coleção GitHub sem lacuna concreta, revisão de origem/licença/scripts e versão fixa.

AGENTS.md, README.md, skills e tooling de governança revisados são candidatos a versionamento. Todo docs/ permanece local/ignorado por decisão explícita do usuário, não apenas imports/auditorias. Não stagear nem forçar inclusão dessa pasta; não alterar sua política para fazer CI passar. Não adicionar licença de produto ou publicar material de terceiros por suposição.

## Git e fechamento

Bootstrap inicial em main no remoto vazio fornecido pelo usuário, com commit/push expressamente autorizados. Entregas de produto posteriores seguem branch curta/PR, CI e autoinspeção conforme D45; sem segundo aprovador obrigatório. Remoto público: Rafael-Albernaz-dev/InfraDesktop; pacote bruto/auditoria continuam ignorados. Não mudar visibilidade nem publicar fontes privadas por consequência. Commits Conventional Commits em inglês. Não herdar branch compartilhada Atlas.

Hermes encerra auditoria com Decision, Evidence, Risks, Required changes, Next command. Codex entrega delta, comandos/exits, provas por gate, limitações e walkthrough numerado. Após qualquer alteração, apresentar output do que foi feito, verificado e ficou pendente. Atualizar contexto/roadmap no mesmo fechamento, sem chamar pendência de PASS.
