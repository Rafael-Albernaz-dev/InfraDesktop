# InfraDesktop

Local desktop operations workspace for ISP technicians. Independent from Atlas; Go/Wails UI, a local Go service and SQLite are the approved product direction. Linux/Ubuntu24.04 comes first; inference remains optional.

## Current status

Documentation and tooling bootstrap; the application has not been implemented. There is no runnable application, installer or enabled service yet. The repository is hosted at https://github.com/Rafael-Albernaz-dev/InfraDesktop.

Shared agent instructions are in [AGENTS.md](AGENTS.md); reusable local procedures are listed in the [skill catalog](.agents/SKILLS.md).

Next: complete host prerequisites and select a disposable Ubuntu environment, then implement the scoped L01 foundation through Codex CLI. No benchmark or operational network support is claimed.

## Documentation policy

The current docs/ directory is a local, ignored preparation workspace containing context, plans, imported references and audit material. None of it is part of this public repository. Official product documentation will be designed separately; the preparation workspace must not be published as a substitute. Public checks must work without docs/.

## Documentation checks

The initial CI is limited to published agent instructions and skills. Run `python3 .agents/tools/check_governance.py` and `python3 -m unittest discover -s .agents/tools -p 'test_*.py' -v` with Python 3.12. These offline checks work without docs/. Application build, architecture and runtime checks belong to the L01 implementation; no application test result is claimed at this stage.

No product license has been selected. Third-party reference material retains its own provenance; do not infer redistribution rights.
