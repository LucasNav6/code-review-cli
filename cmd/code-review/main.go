// Command code-review revisa Pull Requests de GitHub con Claude Code,
// evaluando seguridad OWASP, mantenibilidad, testing y resiliencia.
package main

import "github.com/LucasNav6/code-review-cli/internal/cli"

func main() {
	cli.Execute()
}
