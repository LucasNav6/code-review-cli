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

// resolvePullRequest determines which Pull Request should be reviewed.
// The URL can be provided through --url or entered interactively.
func resolvePullRequest() (*githubpr.PullRequest, error) {
	if urlFlag != "" {
		return githubpr.ParseURL(urlFlag)
	}

	return askPullRequestInteractively()
}

// askPullRequestInteractively requests the Pull Request URL when the CLI
// is running in an interactive terminal and --url was not provided.
func askPullRequestInteractively() (*githubpr.PullRequest, error) {
	if !isInteractiveTerminal() {
		return nil, fmt.Errorf(
			"%s\n\n%s",
			errorStyle.Render("Pull Request required"),
			mutedStyle.Render(
				"Run `code-review --url https://github.com/org/repo/pull/123`.",
			),
		)
	}

	fmt.Println()
	fmt.Println(brandStyle.Render("code-review"))
	fmt.Println()

	if notice := renderUpdateNotice(); notice != "" {
		fmt.Println(notice)
		fmt.Println()
	}

	fmt.Println(
		"Enter a GitHub Pull Request URL",
	)

	fmt.Print(
		brandStyle.Render("> "),
	)

	reader := bufio.NewReader(os.Stdin)

	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf(
			"%s",
			errorStyle.Render("Failed to read Pull Request URL"),
		)
	}

	url := strings.TrimSpace(input)

	if url == "" {
		return nil, fmt.Errorf(
			"%s\n\n%s",
			errorStyle.Render("Pull Request URL is required"),
			mutedStyle.Render(
				"Example: `https://github.com/org/repo/pull/123`",
			),
		)
	}

	pr, err := githubpr.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf(
			"%s\n\n%s",
			errorStyle.Render("Invalid Pull Request URL"),
			mutedStyle.Render(
				"Expected: `https://github.com/org/repo/pull/123`",
			),
		)
	}

	return pr, nil
}

// isInteractiveTerminal reports whether stdin is connected to a terminal.
// This prevents the CLI from waiting for input when running in CI or scripts.
func isInteractiveTerminal() bool {
	fd := os.Stdin.Fd()

	return isatty.IsTerminal(fd) ||
		isatty.IsCygwinTerminal(fd)
}