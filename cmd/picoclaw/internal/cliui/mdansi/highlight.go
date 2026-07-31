package mdansi

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenKind classifies a syntax token for lite highlighting.
type TokenKind int

const (
	KindPlain TokenKind = iota
	KindKeyword
	KindTag
	KindAttr
	KindString
	KindComment
	KindNumber
	KindPunct
	KindIdent
)

// Token is one styled span of source text (may contain newlines).
type Token struct {
	Kind TokenKind
	Text string
}

// Highlight returns a lite token stream for lang. Unknown langs are plain.
func Highlight(lang, src string) []Token {
	lang = NormalizeLang(lang)
	if src == "" {
		return nil
	}
	switch lang {
	case "html", "xml":
		return lexHTML(src)
	case "css":
		return lexCSS(src)
	case "js", "ts":
		return lexJS(src)
	case "go":
		return lexGo(src)
	case "json":
		return lexJSON(src)
	case "shell":
		return lexShell(src)
	case "yaml":
		return lexYAML(src)
	case "md":
		return []Token{{Kind: KindPlain, Text: src}}
	default:
		return []Token{{Kind: KindPlain, Text: src}}
	}
}

// NormalizeLang maps fence aliases to a canonical lite-lexer name.
func NormalizeLang(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "htm", "html", "xml", "svg":
		return "html"
	case "css":
		return "css"
	case "js", "javascript", "ts", "typescript", "tsx", "jsx":
		return "js"
	case "go", "golang":
		return "go"
	case "json", "jsonc":
		return "json"
	case "sh", "bash", "shell", "zsh", "fish":
		return "shell"
	case "yaml", "yml":
		return "yaml"
	case "md", "markdown":
		return "md"
	default:
		return lang
	}
}

func appendTok(out []Token, kind TokenKind, text string) []Token {
	if text == "" {
		return out
	}
	n := len(out)
	if n > 0 && out[n-1].Kind == kind {
		out[n-1].Text += text
		return out
	}
	return append(out, Token{Kind: kind, Text: text})
}

func lexHTML(src string) []Token {
	var out []Token
	i := 0
	for i < len(src) {
		if strings.HasPrefix(src[i:], "<!--") {
			end := strings.Index(src[i:], "-->")
			if end < 0 {
				out = appendTok(out, KindComment, src[i:])
				break
			}
			end += i + 3
			out = appendTok(out, KindComment, src[i:end])
			i = end
			continue
		}
		if src[i] == '<' {
			j := i + 1
			out = appendTok(out, KindPunct, "<")
			if j < len(src) && (src[j] == '/' || src[j] == '!') {
				out = appendTok(out, KindPunct, src[j:j+1])
				j++
			}
			start := j
			for j < len(src) && isIdentByte(src[j]) {
				j++
			}
			if j > start {
				out = appendTok(out, KindTag, src[start:j])
			}
			for j < len(src) && src[j] != '>' {
				if src[j] == '"' || src[j] == '\'' {
					q := src[j]
					k := j + 1
					for k < len(src) && src[k] != q && src[k] != '\n' {
						k++
					}
					if k < len(src) && src[k] == q {
						k++
					}
					out = appendTok(out, KindString, src[j:k])
					j = k
					continue
				}
				if isIdentByte(src[j]) {
					start := j
					for j < len(src) && (isIdentByte(src[j]) || src[j] == '-' || src[j] == ':') {
						j++
					}
					out = appendTok(out, KindAttr, src[start:j])
					continue
				}
				out = appendTok(out, KindPunct, src[j:j+1])
				j++
			}
			if j < len(src) && src[j] == '>' {
				out = appendTok(out, KindPunct, ">")
				j++
			}
			i = j
			continue
		}
		// Plain text until next tag.
		j := i + 1
		for j < len(src) && src[j] != '<' {
			j++
		}
		out = appendTok(out, KindPlain, src[i:j])
		i = j
	}
	return out
}

func lexCSS(src string) []Token {
	return lexCLike(src, cssKeywords, false)
}

func lexJS(src string) []Token {
	return lexCLike(src, jsKeywords, false)
}

func lexGo(src string) []Token {
	return lexCLike(src, goKeywords, true)
}

func lexJSON(src string) []Token {
	var out []Token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '"':
			j := scanString(src, i)
			out = appendTok(out, KindString, src[i:j])
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := scanLineComment(src, i)
			out = appendTok(out, KindComment, src[i:j])
			i = j
		case unicode.IsDigit(rune(c)) || (c == '-' && i+1 < len(src) && unicode.IsDigit(rune(src[i+1]))):
			j := scanNumber(src, i)
			out = appendTok(out, KindNumber, src[i:j])
			i = j
		case isIdentByte(c):
			j := i + 1
			for j < len(src) && isIdentByte(src[j]) {
				j++
			}
			word := src[i:j]
			switch word {
			case "true", "false", "null":
				out = appendTok(out, KindKeyword, word)
			default:
				out = appendTok(out, KindIdent, word)
			}
			i = j
		case strings.ContainsRune("{}[]:,", rune(c)):
			out = appendTok(out, KindPunct, src[i:i+1])
			i++
		default:
			out = appendTok(out, KindPlain, src[i:i+1])
			i++
		}
	}
	return out
}

func lexShell(src string) []Token {
	var out []Token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '#':
			j := scanLineComment(src, i)
			out = appendTok(out, KindComment, src[i:j])
			i = j
		case c == '"' || c == '\'':
			j := scanString(src, i)
			out = appendTok(out, KindString, src[i:j])
			i = j
		case isIdentByte(c):
			j := i + 1
			for j < len(src) && (isIdentByte(src[j]) || src[j] == '-' || src[j] == '_') {
				j++
			}
			word := src[i:j]
			if shellBuiltins[word] {
				out = appendTok(out, KindKeyword, word)
			} else {
				out = appendTok(out, KindIdent, word)
			}
			i = j
		case unicode.IsDigit(rune(c)):
			j := scanNumber(src, i)
			out = appendTok(out, KindNumber, src[i:j])
			i = j
		default:
			out = appendTok(out, KindPlain, src[i:i+1])
			i++
		}
	}
	return out
}

func lexYAML(src string) []Token {
	var out []Token
	lines := strings.SplitAfter(src, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		trim := strings.TrimLeft(line, " \t")
		lead := len(line) - len(trim)
		if lead > 0 {
			out = appendTok(out, KindPlain, line[:lead])
		}
		if strings.HasPrefix(trim, "#") {
			out = appendTok(out, KindComment, trim)
			continue
		}
		// key: value
		colon := strings.IndexByte(trim, ':')
		if colon > 0 && !strings.HasPrefix(trim, "-") {
			key := trim[:colon]
			rest := trim[colon:]
			// Don't treat URLs / times as keys if key has spaces oddly — keep simple.
			if !strings.ContainsAny(key, " \t{}[]") {
				out = appendTok(out, KindAttr, key)
				out = appendTok(out, KindPunct, ":")
				rest = rest[1:]
				out = appendYAMLValue(out, rest)
				continue
			}
		}
		if strings.HasPrefix(trim, "- ") {
			out = appendTok(out, KindPunct, "-")
			out = appendTok(out, KindPlain, " ")
			out = appendYAMLValue(out, trim[2:])
			continue
		}
		out = appendYAMLValue(out, trim)
	}
	return out
}

func appendYAMLValue(out []Token, s string) []Token {
	if s == "" {
		return out
	}
	st := strings.TrimLeft(s, " \t")
	lead := len(s) - len(st)
	if lead > 0 {
		out = appendTok(out, KindPlain, s[:lead])
	}
	if st == "" {
		return out
	}
	if st[0] == '"' || st[0] == '\'' {
		j := scanString(st, 0)
		out = appendTok(out, KindString, st[:j])
		if j < len(st) {
			out = appendTok(out, KindPlain, st[j:])
		}
		return out
	}
	if strings.HasPrefix(st, "#") {
		return appendTok(out, KindComment, st)
	}
	if hash := strings.IndexByte(st, '#'); hash >= 0 {
		out = appendTok(out, KindPlain, st[:hash])
		out = appendTok(out, KindComment, st[hash:])
		return out
	}
	switch st {
	case "true", "false", "null", "yes", "no", "~":
		return appendTok(out, KindKeyword, st)
	}
	if len(st) > 0 && (unicode.IsDigit(rune(st[0])) || st[0] == '-') {
		j := scanNumber(st, 0)
		if j > 0 {
			out = appendTok(out, KindNumber, st[:j])
			if j < len(st) {
				out = appendTok(out, KindPlain, st[j:])
			}
			return out
		}
	}
	return appendTok(out, KindPlain, st)
}

func lexCLike(src string, keywords map[string]bool, rawStrings bool) []Token {
	var out []Token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			j := scanLineComment(src, i)
			out = appendTok(out, KindComment, src[i:j])
			i = j
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				out = appendTok(out, KindComment, src[i:])
				return out
			}
			j = i + 2 + j + 2
			out = appendTok(out, KindComment, src[i:j])
			i = j
		case rawStrings && c == '`':
			j := i + 1
			for j < len(src) && src[j] != '`' {
				j++
			}
			if j < len(src) {
				j++
			}
			out = appendTok(out, KindString, src[i:j])
			i = j
		case c == '"' || c == '\'':
			j := scanString(src, i)
			out = appendTok(out, KindString, src[i:j])
			i = j
		case unicode.IsDigit(rune(c)):
			j := scanNumber(src, i)
			out = appendTok(out, KindNumber, src[i:j])
			i = j
		case isIdentStart(c):
			j := i + 1
			for j < len(src) && isIdentByte(src[j]) {
				j++
			}
			word := src[i:j]
			if keywords[word] {
				out = appendTok(out, KindKeyword, word)
			} else {
				out = appendTok(out, KindIdent, word)
			}
			i = j
		case strings.ContainsRune("{}[]().,;:+-*/%<>=!&|?~^", rune(c)):
			out = appendTok(out, KindPunct, src[i:i+1])
			i++
		default:
			r, size := utf8.DecodeRuneInString(src[i:])
			if r == utf8.RuneError && size == 1 {
				out = appendTok(out, KindPlain, src[i:i+1])
				i++
			} else {
				out = appendTok(out, KindPlain, src[i:i+size])
				i += size
			}
		}
	}
	return out
}

func scanString(src string, i int) int {
	if i >= len(src) {
		return i
	}
	q := src[i]
	j := i + 1
	for j < len(src) {
		if src[j] == '\\' && j+1 < len(src) {
			j += 2
			continue
		}
		if src[j] == q {
			return j + 1
		}
		if src[j] == '\n' && q != '`' {
			return j
		}
		j++
	}
	return j
}

func scanLineComment(src string, i int) int {
	j := i
	for j < len(src) && src[j] != '\n' {
		j++
	}
	return j
}

func scanNumber(src string, i int) int {
	j := i
	if j < len(src) && (src[j] == '+' || src[j] == '-') {
		j++
	}
	for j < len(src) && (unicode.IsDigit(rune(src[j])) || src[j] == '.' || src[j] == '_' ||
		src[j] == 'x' || src[j] == 'X' || src[j] == 'e' || src[j] == 'E') {
		j++
	}
	return j
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isIdentStart(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

var goKeywords = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
	"true": true, "false": true, "iota": true, "nil": true,
}

var jsKeywords = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true,
	"else": true, "export": true, "extends": true, "false": true, "finally": true,
	"for": true, "function": true, "if": true, "import": true, "in": true,
	"instanceof": true, "let": true, "new": true, "null": true, "return": true,
	"super": true, "switch": true, "this": true, "throw": true, "true": true,
	"try": true, "typeof": true, "undefined": true, "var": true, "void": true,
	"while": true, "with": true, "yield": true, "async": true, "await": true,
	"of": true, "from": true, "as": true, "type": true, "interface": true,
}

var cssKeywords = map[string]bool{
	"important": true, "media": true, "keyframes": true, "from": true, "to": true,
	"and": true, "or": true, "not": true, "only": true,
}

var shellBuiltins = map[string]bool{
	"cd": true, "echo": true, "export": true, "if": true, "then": true, "else": true,
	"fi": true, "for": true, "do": true, "done": true, "while": true, "case": true,
	"esac": true, "function": true, "return": true, "local": true, "readonly": true,
	"source": true, "alias": true, "unset": true, "printf": true, "test": true,
	"exit": true, "shift": true, "set": true, "true": true, "false": true,
}
