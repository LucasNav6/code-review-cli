package domain

// Response is the captured output of a Provider.Run call. We wrap
// the raw string in a typed value so future fields (e.g. token
// counts, latency) can be added without breaking every caller.
//
// The Body is the trimmed, raw text the LLM emitted. Parsers (e.g.
// the claudereview JSON parser) operate on the string directly; the
// typed wrapper exists only to make the boundary explicit.
type Response struct {
	// Body is the trimmed LLM output. Implementations are
	// expected to strip leading and trailing whitespace before
	// setting this field so downstream parsers do not have to.
	Body string
}

// NewResponse builds a Response from raw text. The trim is
// intentionally part of the constructor (rather than something each
// adapter does by hand) so every provider gets the same
// whitespace semantics for free.
func NewResponse(raw string) Response {
	return Response{Body: trimSpace(raw)}
}

// trimSpace is a tiny stdlib-free whitespace stripper. Avoiding
// strings.TrimSpace keeps the domain package zero-deps.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace matches the same runes strings.TrimSpace considers
// whitespace: space, tab, newline, vertical tab, form feed,
// carriage return.
func isSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}