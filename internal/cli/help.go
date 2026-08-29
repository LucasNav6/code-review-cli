package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

// renderHelp reemplaza el template de ayuda por defecto de Cobra para
// aplicarle la misma identidad visual que el resto de la CLI: título en
// brand, secciones sin dos puntos, y nombres de comandos/flags en amarillo
// con su descripción en gris.
func renderHelp(cmd *cobra.Command, _ []string) {
	cmd.Print(helpText(cmd))
}

func helpText(cmd *cobra.Command) string {
	var b strings.Builder

	b.WriteString(brandStyle.Render(cmd.CommandPath()))
	b.WriteString("\n")

	if description := strings.TrimSpace(cmd.Long); description != "" {
		b.WriteString(mutedStyle.Render(description))
		b.WriteString("\n")
	} else if cmd.Short != "" {
		b.WriteString(mutedStyle.Render(cmd.Short))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(sectionHeader("Usage"))
	b.WriteString(mutedStyle.Render("  " + cmd.UseLine()))
	b.WriteString("\n")
	if cmd.HasAvailableSubCommands() {
		b.WriteString(mutedStyle.Render("  " + cmd.CommandPath() + " <command>"))
		b.WriteString("\n")
	}

	if cmd.HasExample() {
		b.WriteString("\n")
		b.WriteString(sectionHeader("Examples"))
		b.WriteString(mutedStyle.Render(cmd.Example))
		b.WriteString("\n")
	}

	if cmd.HasAvailableSubCommands() {
		b.WriteString("\n")
		b.WriteString(sectionHeader("Commands"))
		b.WriteString(commandList(cmd))
	}

	if cmd.HasAvailableLocalFlags() {
		b.WriteString("\n")
		b.WriteString(sectionHeader("Flags"))
		b.WriteString(flagList(cmd.LocalFlags()))
	}

	if cmd.HasAvailableInheritedFlags() {
		b.WriteString("\n")
		b.WriteString(sectionHeader("Global Flags"))
		b.WriteString(flagList(cmd.InheritedFlags()))
	}

	if cmd.HasAvailableSubCommands() {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(
			fmt.Sprintf("Use %q for more information about a command.", cmd.CommandPath()+" [command] --help"),
		))
		b.WriteString("\n")
	}

	return b.String()
}

// sectionHeader imprime el título de una sección ("Usage", "Flags", etc.)
// en el mismo estilo brand que el nombre del comando, sin dos puntos.
func sectionHeader(title string) string {
	return brandStyle.Render(title) + "\n"
}

// commandList arma la lista de subcomandos disponibles: nombre en amarillo
// alineado por padding, descripción en gris.
func commandList(cmd *cobra.Command) string {
	commands := cmd.Commands()

	nameWidth := 0
	for _, c := range commands {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		if l := len(c.Name()); l > nameWidth {
			nameWidth = l
		}
	}

	var b strings.Builder
	for _, c := range commands {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		padding := strings.Repeat(" ", nameWidth-len(c.Name()))
		b.WriteString(fmt.Sprintf(
			"  %s%s  %s\n",
			updateStyle.Render(c.Name()),
			padding,
			mutedStyle.Render(c.Short),
		))
	}

	return b.String()
}

// flagList arma la lista de flags de un FlagSet: "--nombre tipo" en
// amarillo alineado por padding, descripción en gris.
func flagList(flags *flag.FlagSet) string {
	type entry struct {
		left  string
		usage string
	}

	var entries []entry
	width := 0

	flags.VisitAll(func(f *flag.Flag) {
		if f.Hidden {
			return
		}

		left := "--" + f.Name
		if f.Shorthand != "" {
			left = "-" + f.Shorthand + ", " + left
		}
		if f.Value.Type() != "bool" {
			left += " " + f.Value.Type()
		}

		if len(left) > width {
			width = len(left)
		}
		entries = append(entries, entry{left: left, usage: f.Usage})
	})

	var b strings.Builder
	for _, e := range entries {
		padding := strings.Repeat(" ", width-len(e.left))
		b.WriteString(fmt.Sprintf(
			"  %s%s  %s\n",
			updateStyle.Render(e.left),
			padding,
			mutedStyle.Render(e.usage),
		))
	}

	return b.String()
}
