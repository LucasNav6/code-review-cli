package update

import (
	"github.com/LucasNav6/code-review-cli/buildinfo"
	"github.com/LucasNav6/code-review-cli/ui"
)

func StatusMessage() string {
	return ui.MutedStyle.Render(
		"update is not wired up yet (current version: " + buildinfo.Version + ")")
}
