# Scripts de Desenvolvimento

Este diretório contém scripts úteis para desenvolvimento do Genus ORM.

## setup-hooks.sh

Configura git hooks para validação de commits.

### Uso:

```bash
./scripts/setup-hooks.sh
```

### O que faz:

- Instala o hook `commit-msg` que valida mensagens de commit
- Valida formato e conteúdo das mensagens de commit
- Garante consistência nas mensagens de commit do projeto

### Quando executar:

- Após clonar o repositório pela primeira vez
- Se os hooks forem removidos ou corrompidos
- Ao atualizar a versão dos hooks (executar novamente sobrescreve)

## Contribuindo

Ao adicionar novos scripts, certifique-se de:

1. Adicionar a extensão `.sh` para scripts shell
2. Torná-los executáveis: `chmod +x scripts/seu-script.sh`
3. Adicionar shebang no início: `#!/bin/bash` ou `#!/bin/sh`
4. Documentar neste README
5. Incluir comentários explicativos no código
