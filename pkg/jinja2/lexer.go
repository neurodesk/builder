package jinja2

// The lexer scans template source and yields tokens for text and the three
// Jinja2 delimiter forms: variables {{ }}, statements {% %}, and comments {# #}.

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokText
	tokVarStart  // {{ or {{-
	tokVarEnd    // }} or -}}
	tokStmtStart // {% or {%-
	tokStmtEnd   // %} or -%}
	tokCommStart // {#
	tokCommEnd   // #}
	tokContent   // content inside a tag (parser requests it)
)

type token struct {
	kind tokenKind
	val  string
	pos  int // byte offset in source
}

type lexer struct {
	src []byte
	i   int
	n   int
}

func newLexer(src []byte) *lexer {
	return &lexer{src: src, n: len(src)}
}

func (l *lexer) next() byte {
	if l.i >= l.n {
		return 0
	}
	b := l.src[l.i]
	l.i++
	return b
}

func (l *lexer) peek() byte {
	if l.i >= l.n {
		return 0
	}
	return l.src[l.i]
}

func (l *lexer) match(s string) bool {
	if l.i+len(s) > l.n {
		return false
	}
	for j := 0; j < len(s); j++ {
		if l.src[l.i+j] != s[j] {
			return false
		}
	}
	l.i += len(s)
	return true
}

// scanUntil scans until the first occurrence of delim and returns the text
// before it. If delim is not found, returns the rest of the input.
func (l *lexer) scanUntil(delim string) (string, bool) {
	start := l.i
	for {
		if l.i >= l.n {
			return string(l.src[start:]), false
		}
		if l.i+len(delim) <= l.n {
			match := true
			for j := 0; j < len(delim); j++ {
				if l.src[l.i+j] != delim[j] {
					match = false
					break
				}
			}
			if match {
				// Return up to but not including delim; do not advance past delim.
				s := string(l.src[start:l.i])
				return s, true
			}
		}
		l.i++
	}
}

// nextToken returns the next token in the stream.
// nextTokenOutside scans in normal text context and emits either a text token
// up to the next opening delimiter, or an opening delimiter token, or EOF.
func (l *lexer) nextTokenOutside() token {
	if l.i >= l.n {
		return token{kind: tokEOF, pos: l.i}
	}

	// Look for any of the starting delimiters.
	// If not at a delimiter, emit text up to the next delimiter or EOF.
	// Check for comment first because it's the most specific.
	start := l.i
	for l.i < l.n {
		if l.i+2 <= l.n {
			switch string(l.src[l.i : l.i+2]) {
			case "{{":
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokText, val: s, pos: start}
				}
				// Consume and emit var start; handle optional trim '-'.
				l.i += 2
				if l.i < l.n && l.src[l.i] == '-' {
					l.i++
				}
				return token{kind: tokVarStart, pos: start}
			case "{%":
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokText, val: s, pos: start}
				}
				l.i += 2
				if l.i < l.n && l.src[l.i] == '-' {
					l.i++
				}
				return token{kind: tokStmtStart, pos: start}
			case "{#":
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokText, val: s, pos: start}
				}
				l.i += 2
				return token{kind: tokCommStart, pos: start}
			}
		}
		l.i++
	}
	// If we fall out, emit the trailing text and then EOF next call.
	if start < l.n {
		s := string(l.src[start:l.n])
		return token{kind: tokText, val: s, pos: start}
	}
	return token{kind: tokEOF, pos: l.i}
}

// nextTokenInside scans inside a tag of the given closing kind, returning
// either tokContent chunks or the appropriate closing token.
func (l *lexer) nextTokenInside(close tokenKind) token {
	if l.i >= l.n {
		return token{kind: tokEOF, pos: l.i}
	}
	start := l.i
	for l.i < l.n {
		if close == tokVarEnd && l.i+2 <= l.n {
			if l.i+3 <= l.n && string(l.src[l.i:l.i+3]) == "-}}" {
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokContent, val: s, pos: start}
				}
				l.i += 3
				return token{kind: tokVarEnd, pos: start}
			}
			if string(l.src[l.i:l.i+2]) == "}}" {
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokContent, val: s, pos: start}
				}
				l.i += 2
				return token{kind: tokVarEnd, pos: start}
			}
		}
		if close == tokStmtEnd && l.i+2 <= l.n {
			if l.i+3 <= l.n && string(l.src[l.i:l.i+3]) == "-%}" {
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokContent, val: s, pos: start}
				}
				l.i += 3
				return token{kind: tokStmtEnd, pos: start}
			}
			if string(l.src[l.i:l.i+2]) == "%}" {
				if l.i > start {
					s := string(l.src[start:l.i])
					return token{kind: tokContent, val: s, pos: start}
				}
				l.i += 2
				return token{kind: tokStmtEnd, pos: start}
			}
		}
		if close == tokCommEnd && l.i+2 <= l.n && string(l.src[l.i:l.i+2]) == "#}" {
			if l.i > start {
				s := string(l.src[start:l.i])
				return token{kind: tokContent, val: s, pos: start}
			}
			l.i += 2
			return token{kind: tokCommEnd, pos: start}
		}
		l.i++
	}
	// Unterminated tag; return remaining content then EOF.
	if start < l.n {
		s := string(l.src[start:l.n])
		return token{kind: tokContent, val: s, pos: start}
	}
	return token{kind: tokEOF, pos: l.i}
}
