# code-review

CLI que revisa Pull Requests de GitHub, organizada en etapas independientes:

- **Seguridad** — OWASP API Security Top 10 2023 (vía Claude Code)
- **Dependencias** — vulnerabilidades conocidas en las dependencias del PR, escaneadas contra [OSV.dev](https://osv.dev) con [OSV-Scanner](https://github.com/google/osv-scanner) embebido. A diferencia de las demás etapas, es 100% determinística: no pasa por ningún modelo de lenguaje.
- **Mantenibilidad** — estructura, legibilidad y complejidad (vía Claude Code)
- **Testing** — cobertura y casos borde (vía Claude Code)
- **Resiliencia** — manejo de fallos y observabilidad (vía Claude Code)

Muestra el progreso de cada etapa en una interfaz interactiva de terminal y deja los hallazgos de cada una en un archivo Markdown en el directorio actual.

## Tipos de revisión

El LLM revisa el diff con cuatro lentes distintos. Cada uno corre con
un prompt propio (`internal/prompts/<tipo>.md`) y devuelve hallazgos
en una categoría (`RESILIENCE`, `READABILITY`, `SECURITY` o `TESTING`).
El parser y el renderer son compartidos: lo único que cambia es el
prompt y los campos extra que cada categoría muestra.

**Por defecto se corren los cuatro**, en este orden: resilience →
maintainability → security → testing. Cada uno con su propio spinner
y su propio bloque de hallazgos, identificados por el header
`─── Claude review (<categoría>) ───`. Si uno de los cuatro falla, los
otros siguen — el flujo no se aborta.

| `--type`            | Categoría   | Busca                                                             | Campos extra                     |
| ------------------- | ----------- | ----------------------------------------------------------------- | -------------------------------- |
| `resilience`        | `RESILIENCE`| Manejo de fallos, retries, timeouts, observabilidad                | —                                |
| `maintainability`   | `READABILITY`| Números mágicos, complejidad, duplicación, nombres              | —                                |
| `security`          | `SECURITY`  | OWASP API Top 10 (BOLA, BFLA, SSRF, misconfig, etc.)             | `owasp` (ej. `API1:2023`)        |
| `testing`           | `TESTING`   | Cobertura de tests, casos borde, regresiones                     | `requires_tests`, `tests_covered`, `tests_missing`, `edge_case` |
| `all` (def.)        | las cuatro  | Lo mismo que las cuatro anteriores, en orden                      | según corresponda                |

```sh
code-review --url https://github.com/org/repo/pull/123                # default: las cuatro categorías
code-review --url <pr> --type security                                # solo OWASP (más rápido, menos tokens)
code-review --url <pr> --type testing                                 # solo cobertura + casos borde
code-review --url <pr> --type all                                     # explícito: los cuatro en orden
```

## Requisitos

- [`gh`](https://cli.github.com) (GitHub CLI), autenticado con `gh auth login`.
- [`claude`](https://docs.claude.com/claude-code) (Claude Code CLI), autenticado.

No hace falta instalar nada aparte para el escaneo de dependencias — OSV-Scanner viaja embebido dentro del binario de `code-review`.

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

# Bajar a una versión específica
code-review downgrade --to v1.0.0

# Desinstalar el binario
code-review uninstall
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

La CLI trae tres comandos para mantener el binario bajo control. Los
tres hablan contra las [GitHub Releases][releases] del repo y funcionan
tanto si instalaste con el script oficial como con `go install`.

### `code-review upgrade`

Baja la última release estable publicada y reemplaza el binario actual
en disco. Antes de tocar el filesystem pide confirmación (salteable con
`-y`). Si ya estás en la última versión, sale con un mensaje y exit 0.

```sh
code-review upgrade          # pregunta antes de pisar
code-review upgrade -y       # upgrade silencioso para scripts/CI
code-review upgrade --help
```

`update` queda como alias por compatibilidad hacia atrás.

### `code-review downgrade --to <version>`

Baja una release específica y reemplaza el binario actual. **Requiere
que la versión target sea estrictamente menor** que la que está corriendo
— si pedís `--to v2.0.0` desde `v1.5.0` el comando rechaza la operación
con un mensaje claro en vez de hacer un "upgrade disfrazado".

```sh
code-review downgrade --to v1.0.0
code-review downgrade --to v1.2.3
```

### `code-review uninstall`

Detecta cómo se instaló el binario y lo borra cuando es seguro:

| Instalado en              | Qué hace                                                |
| ------------------------- | ------------------------------------------------------- |
| `~/.local/bin/code-review`| Borra el archivo en disco.                              |
| `$GOBIN` / `$GOPATH/bin`  | Borra el archivo en disco.                              |
| Homebrew cellar           | Imprime `brew uninstall code-review` y no toca nada.    |
| Cualquier otro lugar      | Imprime la instrucción manual y no toca nada.           |

Pide confirmación antes de borrar (salteable con `-y`). Self-update y
uninstall **no funcionan en Windows** — la estrategia de rename
atómico no aplica sobre un binario en ejecución.

[releases]: https://github.com/LucasNav6/code-review-cli/releases

## Desarrollo

```sh
make check   # fmt + vet + test
make build   # compila a bin/code-review con info de versión
```

### Logger

```go
return logging.LogError(os.Stderr, logging.ErrorTypeInput, 1, err)
logging.LogInfo(os.Stderr, "diff stored", "path", path, "bytes", size)
logging.LogDebug(os.Stderr, "running gh", "command", "gh pr diff", "url", url)
logging.LogWarn(os.Stderr, "cache fallback", "dir", dir)
```

Salida:

```text
ERRO (12:34) Exit status (1)
╰─▶ pull request URL is required

INFO (12:34) diff stored
 | path=/tmp/review.diff
 | bytes=2048
```

### Releases

Los binarios se publican automáticamente vía [GoReleaser](https://goreleaser.com) al pushear un tag `vX.Y.Z`:

```sh
git tag v0.1.0
git push origin v0.1.0
```

El workflow en `.github/workflows/release.yml` compila binarios para Linux/macOS/Windows (amd64 y arm64) y los adjunta a una GitHub Release, junto con un `checksums.txt`. Solo se publican releases de tags cuyo commit pertenece a `master`.

## Contribuir

Los cambios se aceptan únicamente vía Pull Request contra `master`, y requieren aprobación de un code owner (ver [`.github/CODEOWNERS`](.github/CODEOWNERS)). Guía completa en [`.github/CONTRIBUTING.md`](.github/CONTRIBUTING.md).

## Licencia

[Apache License 2.0](./LICENSE).
