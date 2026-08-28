package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/LucasNav6/code-review-cli/internal/githubpr"
	"github.com/LucasNav6/code-review-cli/internal/ui"
)

func runReview(cmd *cobra.Command, args []string) error {
	pr, err := resolvePullRequest()
	if err != nil {
		return err
	}

	if err := ui.Run(*pr); err != nil {
		return err
	}

	printUpdateNoticeIfAny()

	return nil
}

// resolvePullRequest determina qué Pull Request revisar, en orden de
// prioridad: --url, --org/--repo/--id, o el modo interactivo que pregunta
// la URL por stdin.
func resolvePullRequest() (*githubpr.PullRequest, error) {
	if urlFlag != "" {
		return githubpr.ParseURL(urlFlag)
	}

	if orgFlag != "" || repoFlag != "" || idFlag != 0 {
		if orgFlag == "" || repoFlag == "" || idFlag == 0 {
			return nil, fmt.Errorf(
				"para identificar un pull request con --org/--repo/--id necesito los tres valores",
			)
		}

		return githubpr.New(orgFlag, repoFlag, idFlag), nil
	}

	return askPullRequestInteractively()
}

func askPullRequestInteractively() (*githubpr.PullRequest, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return nil, fmt.Errorf(
			"no se especificó ningún pull request. Usá --url=\"https://github.com/org/repo/pull/123\" o --org/--repo/--id",
		)
	}

	fmt.Println()
	fmt.Println(brandStyle.Render("code-review"))
	fmt.Println(mutedStyle.Render("Revisión automática de código con Claude"))
	fmt.Println()
	fmt.Println("Necesito la URL del pull request para analizar:")
	fmt.Print(brandStyle.Render("> "))

	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("no pude leer la URL")
	}

	return githubpr.ParseURL(strings.TrimSpace(input))
}
