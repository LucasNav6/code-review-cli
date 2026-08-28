# code-review

CLI que revisa Pull Requests de GitHub con [Claude Code](https://docs.claude.com/claude-code), organizada en cuatro etapas independientes:

- **Seguridad** — OWASP API Security Top 10 2023
- **Mantenibilidad** — estructura, legibilidad y complejidad
- **Testing** — cobertura y casos borde
- **Resiliencia** — manejo de fallos y observabilidad

Muestra el progreso de cada etapa en una interfaz interactiva de terminal y deja los hallazgos de cada una en un archivo Markdown en el directorio actual.

## Requisitos

- [`gh`](https://cli.github.com) (GitHub CLI), autenticado con `gh auth login`.
- [`claude`](https://docs.claude.com/claude-code) (Claude Code CLI), autenticado.

## Instalación

### Sin Go (binario precompilado)

```sh
curl -fsSL https://raw.githubusercontent.com/LucasNav6/code-review-cli/master/scripts/install.sh \
  | CODE_REVIEW_REPO=LucasNav6/code-review-cli sh
```

O descargá el binario para tu plataforma directamente desde la página de [Releases](https://github.com/LucasNav6/code-review-cli/releases).

### Con Go

```sh
go install github.com/LucasNav6/code-review-cli/cmd/code-review@latest
```

## Uso

```sh
# A partir de la URL del pull request
code-review --url="https://github.com/org/repo/pull/123"

# A partir de organización, repositorio y número
code-review --org="org" --repo="repo" --id=123

# Modo interactivo (pregunta la URL)
code-review

# Ayuda y versión
code-review --help
code-review --version

# Actualizar a la última versión publicada
code-review upgrade
```

### Controles de la interfaz

| Tecla       | Acción                          |
| ----------- | -------------------------------- |
| `←` `→`     | Cambiar de etapa de revisión     |
| `↑` `↓`     | Navegar hallazgos / hacer scroll |
| `Tab`       | Alternar entre hallazgos y salida cruda de Claude |
| `PgUp/PgDn` | Paginar la salida de Claude      |
| `q`         | Salir                            |

## Actualizaciones

`code-review` chequea (con una caché de 24hs) si hay una versión más nueva publicada y, si la hay, muestra un aviso al terminar una revisión. Para actualizar:

```sh
code-review upgrade
```

## Desarrollo

```sh
make check   # fmt + vet + test
make build   # compila a bin/code-review con info de versión
```

### Releases

Los binarios se publican automáticamente vía [GoReleaser](https://goreleaser.com) al pushear un tag `vX.Y.Z`:

```sh
git tag v0.1.0
git push origin v0.1.0
```

El workflow en `.github/workflows/release.yml` compila binarios para Linux/macOS/Windows (amd64 y arm64) y los adjunta a una GitHub Release, junto con un `checksums.txt`.
