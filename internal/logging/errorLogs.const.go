package logging

type ErrorType string

const (
	ErrorTypeUnknown                ErrorType = "unknown"
	ErrorTypeInput                  ErrorType = "input"
	ErrorTypeCommandFlag            ErrorType = "command_flag"
	ErrorTypeGitHubCLINotInstalled  ErrorType = "github_cli_not_installed"
	ErrorTypeGitHubCLIUnavailable   ErrorType = "github_cli_unavailable"
	ErrorTypeGitHubCLIInvalidOutput ErrorType = "github_cli_invalid_output"
	ErrorTypeGitHubAuth             ErrorType = "github_auth"
	ErrorTypeGitHubPullRequest      ErrorType = "github_pull_request"
	ErrorTypeDiffFetch              ErrorType = "diff_fetch"
	ErrorTypeDiffStore              ErrorType = "diff_store"
	ErrorTypeMetadata               ErrorType = "metadata_fetch"
)
