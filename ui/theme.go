// Package ui hosts all presentation concerns of the CLI: the colour
// palette (theme.go), the styles derived from it (styles.go), and any
// future rendering helpers. Anything that produces bytes for the user
// (error rendering, banner, progress…) belongs here.
//
// We deliberately keep the palette small and semantic. Names refer to
// the role a colour plays (Danger, Hint, Muted), not the visual hue.
// The same name across operations means the same intent, even if the
// hex value shifts during a redesign.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// ANSI / hex values for every semantic colour. Exposed as constants so
// external packages can quote them in docs or tests without having to
// import the unexported Palette.
const (
	// Fg is the default foreground (high-contrast text).
	Fg = "#FAFAFA"

	// Bg is the default background. Most CLI users never see this, but
	// it is still useful for banners or panels.
	Bg = "#09090B"

	// Muted is for secondary text (descriptions, examples, units).
	Muted = "#8B949E"

	// Dim is for very low-emphasis text (placeholders, hints).
	Dim = "#6E7681"

	// Border is for box outlines when the design needs a frame.
	Border = "#30363D"

	// Brand is the project's identity colour (warm amber).
	// Reserved for names like the application brand mark or section
	// titles in the help text.
	Brand = "#F59E0B"

	// Accent is for highlights (selected item, focus, key bindings).
	Accent = "#58A6FF"

	// Success conveys "everything is fine" (clean stage, no findings).
	Success = "#3FB950"

	// Warning conveys "watch out" (deprecated flag, soft error).
	Warning = "#D29922"

	// Danger conveys "this is wrong" (errors, blockers).
	Danger = "#F85149"

	// Hint conveys "here is how to fix it" — used after Danger to render
	// the actionable suggestion attached to a sentinel error.
	Hint = "#A5D6FF"

	// BranchBg / BranchFg are the pair used to render branch name
	// pills (e.g. [master], [hotfix/...]) inside the PR header.
	BranchBg = "#13233A"

	// BranchFg is the text colour on top of BranchBg. It deliberately
	// matches Accent so branches read as "named identifiers" — the same
	// visual treatment as code identifiers elsewhere in the UI.
	BranchFg = "#58A6FF"
)

// colorFromHex turns a hex string into a color.Color. lipgloss v2
// already accepts this value in Foreground / Background, so the indirection
// only exists to keep the Palette API typed instead of stringly-typed.
func colorFromHex(hex string) color.Color {
	return lipgloss.Color(hex)
}

// Palette groups every semantic colour of the theme as a color.Color.
// Keeping them in a struct (rather than as a flat list of vars) makes
// it obvious which colour is which at the call site:
//
//	ui.Default.Danger.Foreground(...)
//
// instead of a sea of free variables.
type Palette struct {
	Fg                 color.Color
	Bg                 color.Color
	Muted, Dim, Border color.Color
	Brand, Accent      color.Color
	Success, Warning   color.Color
	Danger, Hint       color.Color
	BranchBg, BranchFg color.Color
}

// Default is the project-wide palette. It is populated once at package
// init time and shared by every caller — colours are immutable for the
// lifetime of the process so the singleton model is safe.
var Default = Palette{
	Fg:       colorFromHex(Fg),
	Bg:       colorFromHex(Bg),
	Muted:    colorFromHex(Muted),
	Dim:      colorFromHex(Dim),
	Border:   colorFromHex(Border),
	Brand:    colorFromHex(Brand),
	Accent:   colorFromHex(Accent),
	Success:  colorFromHex(Success),
	Warning:  colorFromHex(Warning),
	Danger:   colorFromHex(Danger),
	Hint:     colorFromHex(Hint),
	BranchBg: colorFromHex(BranchBg),
	BranchFg: colorFromHex(BranchFg),
}
