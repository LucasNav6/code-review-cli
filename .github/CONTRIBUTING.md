# Contribuir a code-review

Gracias por el interés en contribuir. Este proyecto acepta cambios
**únicamente vía Pull Request** — no se aceptan pushes directos a `master`.

## Flujo de trabajo

1. Forkeá el repositorio (o creá una rama si tenés acceso de escritura).
2. Creá una rama descriptiva a partir de `master`, por ejemplo
   `fix/parseo-markdown` o `feat/soporte-gitlab`.
3. Hacé los cambios. Si tocás lógica no trivial, sumá o actualizá tests.
4. Antes de abrir el PR, corré localmente:

   ```sh
   make check   # gofmt + go vet + go test
   ```

5. Abrí el Pull Request contra `master` con una descripción clara de
   **qué** cambia y **por qué**.
6. Todo PR requiere revisión y aprobación de un code owner (ver
   [`CODEOWNERS`](./CODEOWNERS)) antes de poder mergearse.
7. Mergeamos con "Squash and merge" para mantener el historial de
   `master` limpio.

## Lineamientos de código

- Seguí la estructura de paquetes existente (`internal/<dominio>`); evitá
  agregar lógica de dominio dentro de `internal/cli` o `internal/ui`.
- Priorizá funciones chicas y testeables por sobre abstracciones nuevas.
- No agregues dependencias externas sin justificarlo en el PR.
- Los mensajes de usuario (TUI, errores, `--help`) van en español de
  Argentina, cordial y profesional — igual que el resto del proyecto.

## Reportar bugs o proponer features

Abrí un issue describiendo el comportamiento actual, el esperado, y pasos
para reproducirlo si aplica. Para features, contá el caso de uso antes de
implementar — evita PRs grandes que después no se puedan mergear.

## Licencia

Este proyecto está bajo la [licencia Apache 2.0](../LICENSE). Al enviar un
Pull Request, aceptás que tu contribución se distribuya bajo esos mismos
términos.
