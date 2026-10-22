---
name: infradesktop-audit
description: Use when auditing InfraDesktop delivery evidence.
version: 0.1.0
platforms: [linux]
---
# Auditoria InfraDesktop

## Quando usar

Revisar retorno Codex, arquitetura, segurança ou ciclo de vida. Não usar para executar instalações/rede/serviços sem autorização.

## Procedimento

1. Ler AGENTS.md, contexto.md, prompt ativo e evidência entregue. Mapear cada requisito a alegação/prova/pendência; instrução atual prevalece. Não aceitar resumo de executor como prova.
2. Inspecionar estado real com read_file/search_files/terminal: branch/HEAD/diff/staged/untracked se Git existir; runtime/PID/cwd somente se necessário. Preservar mudanças de terceiros, serviços humanos e fonte original.
3. Traçar entrada → autorização/caso de uso → adapter → resultado. ARQ-001 exige domínio/aplicação sem tecnologia, composição explícita e revisão de responsabilidades. Inspecionar grafo transitivo e testes negativos do guard; árvore de pastas não basta.
4. Conferir segurança proporcional: autenticação de pares e aplicação, perfil/ativação, limites/deadlines/cliente lento, erro sanitizado e cleanup próprio. Sintético não prova conta/cofre/firmware; mesmo UID/root são ameaças distintas.
5. Reutilizar provas válidas e rodar somente teste focal autorizado invalidado pelo delta. Diferenciar documental/unitário/integração/Wails real/instalação/boot/equipamento/aceite humano. Ausente ou suspenso continua pendente.
6. Classificar achado comprovado, hipótese ou não especificado e momento antes init/build/durante slice/posterior. Corrigir no mesmo ciclo autorizado; não reabrir D61–D62 ou M03 por rotina.
7. Fechar com Decision, Evidence, Risks, Required changes, Next command; arquivar detalhes sanitizados em docs/evidencias e reconciliar contexto/Roadmap. Não commitar/publicar por consequência do parecer.

## Armadilhas

Corepack pode baixar pacote até em --version: inspecionar shims e desabilitar rede em preflight sem instalação. Verificar comandos efetivamente executados e seus exits. Doctor não é build, browser não é Wails e processo vivo não é boot antes do login. Não homologa4GB em workstation.

## Verificação

Nenhum aceite integral com gate obrigatório pendente; nenhuma alegação operacional sem saída real. Rastreabilidade requisito → prova, identificação da limitação e escopo de ARQ-001 presentes. Informar explicitamente efeitos incidentais.
