package ui

import (
	"path/filepath"
	"strings"
	"unicode"
)

func highlightCode(path string, code string) string {
	keywords := keywordsForPath(path)
	if len(keywords) == 0 {
		return code
	}

	commentPrefix := commentPrefixForPath(path)
	if commentPrefix != "" {
		trimmed := strings.TrimSpace(code)
		if strings.HasPrefix(trimmed, commentPrefix) {
			return syntaxCommentStyle.Render(code)
		}
	}

	var b strings.Builder

	for i := 0; i < len(code); {
		ch := rune(code[i])

		if ch == '"' || ch == '\'' || ch == '`' {
			token, next := readQuoted(code, i, byte(ch))
			b.WriteString(syntaxStringStyle.Render(token))
			i = next
			continue
		}

		if isIdentifierStart(ch) {
			token, next := readIdentifier(code, i)
			if keywords[token] {
				b.WriteString(syntaxKeywordStyle.Render(token))
			} else if isLiteral(token) {
				b.WriteString(syntaxLiteralStyle.Render(token))
			} else {
				b.WriteString(token)
			}
			i = next
			continue
		}

		if unicode.IsDigit(ch) {
			token, next := readNumber(code, i)
			b.WriteString(syntaxNumberStyle.Render(token))
			i = next
			continue
		}

		b.WriteByte(code[i])
		i++
	}

	return b.String()
}

func keywordsForPath(path string) map[string]bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return keywordSet("break default func interface select case defer go map struct chan else goto package switch const fallthrough if range type continue for import return var")
	case ".js", ".jsx", ".ts", ".tsx":
		return keywordSet("as async await break case catch class const continue debugger default delete do else export extends finally for from function get if import in instanceof let new of return set static super switch this throw try typeof var void while with yield")
	case ".php":
		return keywordSet("abstract and array as break callable case catch class clone const continue declare default die do echo else elseif empty enddeclare endfor endforeach endif endswitch endwhile eval exit extends final finally fn for foreach function global goto if implements include include_once instanceof insteadof interface isset list namespace new or print private protected public readonly require require_once return static switch throw trait try unset use var while xor yield")
	case ".py":
		return keywordSet("and as assert async await break class continue def del elif else except false finally for from global if import in is lambda none nonlocal not or pass raise return true try while with yield")
	case ".java", ".kt", ".kts":
		return keywordSet("abstract as assert boolean break byte case catch char class companion const continue default do double else enum extends false final finally float for fun if implements import in instanceof int interface is long new null object override package private protected public return short static super switch this throw throws true try val var void when while")
	case ".rb":
		return keywordSet("alias and begin break case class def defined do else elsif end ensure false for if in module next nil not or redo rescue retry return self super then true undef unless until when while yield")
	case ".rs":
		return keywordSet("as async await break const continue crate dyn else enum extern false fn for if impl in let loop match mod move mut pub ref return self Self static struct super trait true type unsafe use where while")
	case ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp":
		return keywordSet("auto bool break case catch char class const constexpr continue default delete do double else enum extern false float for friend if inline int long namespace new nullptr private protected public return short signed sizeof static struct switch template this throw true try typedef typename union unsigned using virtual void volatile while")
	case ".css", ".scss", ".sass":
		return keywordSet("important media supports keyframes import charset namespace page font-face layer container")
	case ".html", ".xml", ".vue", ".svelte":
		return keywordSet("html head body div span script style template section article main header footer nav button input form class id")
	case ".json", ".yaml", ".yml":
		return keywordSet("true false null")
	default:
		return nil
	}
}

func keywordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

func commentPrefixForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".py", ".rb", ".sh", ".bash", ".zsh", ".yaml", ".yml":
		return "#"
	case ".html", ".xml", ".vue", ".svelte":
		return "<!--"
	default:
		return "//"
	}
}

func readQuoted(value string, start int, quote byte) (string, int) {
	for i := start + 1; i < len(value); i++ {
		if value[i] == '\\' {
			i++
			continue
		}

		if value[i] == quote {
			return value[start : i+1], i + 1
		}
	}

	return value[start:], len(value)
}

func readIdentifier(value string, start int) (string, int) {
	for i := start; i < len(value); i++ {
		ch := rune(value[i])
		if !isIdentifierPart(ch) {
			return value[start:i], i
		}
	}

	return value[start:], len(value)
}

func readNumber(value string, start int) (string, int) {
	for i := start; i < len(value); i++ {
		ch := rune(value[i])
		if !unicode.IsDigit(ch) && ch != '.' && ch != '_' {
			return value[start:i], i
		}
	}

	return value[start:], len(value)
}

func isIdentifierStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_' || ch == '$'
}

func isIdentifierPart(ch rune) bool {
	return isIdentifierStart(ch) || unicode.IsDigit(ch) || ch == '-'
}

func isLiteral(token string) bool {
	switch token {
	case "true", "false", "nil", "null", "None", "True", "False":
		return true
	default:
		return false
	}
}
