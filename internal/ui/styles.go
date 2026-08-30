package ui

import "charm.land/lipgloss/v2"

// Paleta dark inspirada en GitHub: fondo profundo, bordes suaves y acentos
// mínimos para estado, branch chips y líneas de diff.
var (
	fg              = lipgloss.Color("#FAFAFA")
	bg              = lipgloss.Color("#09090B")
	muted           = lipgloss.Color("#8B949E")
	dim             = lipgloss.Color("#6E7681")
	border          = lipgloss.Color("#30363D")
	strongBorder    = lipgloss.Color("#58A6FF")
	panelBg         = lipgloss.Color("#0D1117")
	headerBg        = lipgloss.Color("#161B22")
	addedBg         = lipgloss.Color("#12261F")
	deletedBg       = lipgloss.Color("#2D1517")
	commentLine     = lipgloss.Color("#8B949E")
	commentBg       = lipgloss.Color("#0B1220")
	commentHeaderBg = lipgloss.Color("#13233A")
	commentFg       = lipgloss.Color("#D6E2FF")
	mergedBg        = lipgloss.Color("#8957E5")
	branchBg        = lipgloss.Color("#13233A")
	branchFg        = lipgloss.Color("#58A6FF")
	severityAccent  = lipgloss.Color("#F0883E")

	// primary queda como alias de fg: lo usa el spinner, que antes tomaba
	// el color de marca directamente.
	primary = fg

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	brandStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	brandBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(bg).
			Background(fg).
			Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
			Foreground(muted)

	dimStyle = lipgloss.NewStyle().
			Foreground(dim)

	// successStyle es texto normal: el símbolo (✓) ya transmite el estado,
	// no hace falta un color aparte.
	successStyle = lipgloss.NewStyle().
			Foreground(fg)

	// warningStyle y errorStyle usan negrita para llamar la atención sin
	// recurrir a un acento de color.
	warningStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	infoStyle = lipgloss.NewStyle().
			Foreground(muted)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	normalPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(border).
			Background(panelBg).
			Padding(0, 1)

	activePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(strongBorder).
			Background(panelBg).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	// tabIndicatorStyle pinta la línea debajo de la etapa seleccionada en
	// la fila de tabs del header.
	tabIndicatorStyle = lipgloss.NewStyle().
				Foreground(strongBorder)

	fileStyle = lipgloss.NewStyle().
			Foreground(muted)

	categoryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(severityAccent)

	// badgeStyle pinta el contador de hallazgos "[N]" de cada tab.
	badgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(severityAccent)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg)

	prStateStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg).
			Background(mergedBg).
			Padding(0, 1)

	branchStyle = lipgloss.NewStyle().
			Foreground(branchFg).
			Background(branchBg).
			Padding(0, 1)

	fileHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(fg).
			Background(headerBg).
			Padding(0, 1)

	hunkStyle = lipgloss.NewStyle().
			Foreground(muted).
			Background(branchBg)

	contextLineStyle = lipgloss.NewStyle().
				Foreground(fg)

	addedLineStyle = lipgloss.NewStyle().
			Foreground(fg).
			Background(addedBg)

	deletedLineStyle = lipgloss.NewStyle().
				Foreground(fg).
				Background(deletedBg)

	additionStatStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#3FB950"))

	deletionStatStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F85149"))

	syntaxKeywordStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF79C6"))

	syntaxStringStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F1FA8C"))

	syntaxCommentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6272A4"))

	syntaxNumberStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#BD93F9"))

	syntaxLiteralStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8BE9FD"))

	commentTargetStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(commentLine).
				Background(lipgloss.Color("#3B2E16")).
				PaddingLeft(1)

	commentPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(strongBorder).
			Foreground(commentFg).
			Background(commentBg).
			Padding(0, 1)

	commentLineStyle = lipgloss.NewStyle().
				Foreground(dim)

	commentHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#CDE3FF")).
			Background(commentHeaderBg).
			Padding(0, 1)

	commentCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(severityAccent)

	commentBodyStyle = lipgloss.NewStyle().
			Foreground(commentFg)

	commentMutedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#8EA4C8"))
)
