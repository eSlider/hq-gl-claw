package mdansi

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// OutputMode selects terminal styling dialect.
type OutputMode int

const (
	// ModeANSI emits SGR escape sequences (stdout / LiveDisplay).
	ModeANSI OutputMode = iota
	// ModeGotui emits termui/gotui markup: [text](fg:cyan,mod:bold).
	ModeGotui
)

const (
	sgrReset   = "\x1b[0m"
	sgrBold    = "\x1b[1m"
	sgrItalic  = "\x1b[3m"
	sgrCode    = "\x1b[36m"
	sgrHead    = "\x1b[1;36m"
	sgrLink    = "\x1b[34m"
	sgrQuote   = "\x1b[3;90m"
	sgrStrike  = "\x1b[9m"
	sgrKeyword = "\x1b[36m"   // cyan
	sgrString  = "\x1b[32m"   // green
	sgrComment = "\x1b[3;90m" // italic grey
	sgrAttr    = "\x1b[33m"   // yellow
	sgrNumber  = "\x1b[35m"   // magenta
	sgrPunct   = "\x1b[90m"   // dim
)

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// Render converts markdown to ANSI-styled terminal text wrapped to width.
func Render(markdown string, width int) string {
	return RenderMode(markdown, width, ModeANSI)
}

// RenderGotui converts markdown to gotui/termui styled markup wrapped to width.
func RenderGotui(markdown string, width int) string {
	return RenderMode(markdown, width, ModeGotui)
}

// RenderMode converts markdown using the given style dialect.
func RenderMode(markdown string, width int, mode OutputMode) string {
	if markdown == "" {
		return ""
	}
	if width < 20 {
		width = 20
	}

	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	nr := &ansiRenderer{width: width, mode: mode}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(
					util.Prioritized(nr, 1000),
				),
			),
		),
	)
	if err := md.Convert([]byte(markdown), buf); err != nil {
		return markdown
	}
	out := strings.TrimRight(buf.String(), "\n")
	if mode == ModeANSI {
		return wrapANSI(out, width)
	}
	return wrapPlain(out, width)
}

type ansiRenderer struct {
	width      int
	mode       OutputMode
	listDepth  int
	orderedIdx []int
	styles     []string // gotui style stack (e.g. "mod:bold")
}

func (r *ansiRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.renderPass)
	reg.Register(ast.KindHeading, r.renderHeading)
	reg.Register(ast.KindParagraph, r.renderParagraph)
	reg.Register(ast.KindTextBlock, r.renderParagraph)
	reg.Register(ast.KindText, r.renderText)
	reg.Register(ast.KindString, r.renderString)
	reg.Register(ast.KindEmphasis, r.renderEmphasis)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCode)
	reg.Register(ast.KindList, r.renderList)
	reg.Register(ast.KindListItem, r.renderListItem)
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
	reg.Register(ast.KindBlockquote, r.renderBlockquote)
	reg.Register(ast.KindThematicBreak, r.renderHR)
	reg.Register(ast.KindRawHTML, r.renderSkip)
	reg.Register(ast.KindHTMLBlock, r.renderSkip)
	reg.Register(east.KindTable, r.renderTable)
	reg.Register(east.KindTableHeader, r.renderPass)
	reg.Register(east.KindTableRow, r.renderTableRow)
	reg.Register(east.KindTableCell, r.renderTableCell)
	reg.Register(east.KindStrikethrough, r.renderStrike)
}

func (r *ansiRenderer) renderPass(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderSkip(util.BufWriter, []byte, ast.Node, bool) (ast.WalkStatus, error) {
	return ast.WalkSkipChildren, nil
}

func (r *ansiRenderer) pushGotui(style string) {
	r.styles = append(r.styles, style)
}

func (r *ansiRenderer) popGotui() {
	if n := len(r.styles); n > 0 {
		r.styles = r.styles[:n-1]
	}
}

func (r *ansiRenderer) writeStyled(w util.BufWriter, text string) {
	if text == "" {
		return
	}
	if r.mode == ModeGotui && len(r.styles) > 0 {
		style := strings.Join(r.styles, ",")
		text = strings.ReplaceAll(text, "]", "〉")
		_, _ = w.WriteString(fmt.Sprintf("[%s](%s)", text, style))
		return
	}
	_, _ = w.WriteString(text)
}

func (r *ansiRenderer) openANSI(w util.BufWriter, sgr string) {
	if r.mode == ModeANSI {
		_, _ = w.WriteString(sgr)
	}
}

func (r *ansiRenderer) closeANSI(w util.BufWriter) {
	if r.mode == ModeANSI {
		_, _ = w.WriteString(sgrReset)
	}
}

func (r *ansiRenderer) renderHeading(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if r.mode == ModeGotui {
			r.pushGotui("fg:cyan,mod:bold")
		} else {
			r.openANSI(w, sgrHead)
		}
	} else {
		if r.mode == ModeGotui {
			r.popGotui()
		} else {
			r.closeANSI(w)
		}
		_, _ = w.WriteString("\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderParagraph(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderText(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	t := n.(*ast.Text)
	r.writeStyled(w, string(t.Segment.Value(source)))
	if t.SoftLineBreak() || t.HardLineBreak() {
		_ = w.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderString(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	r.writeStyled(w, string(n.(*ast.String).Value))
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderEmphasis(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	em := n.(*ast.Emphasis)
	if entering {
		if em.Level >= 2 {
			if r.mode == ModeGotui {
				r.pushGotui("mod:bold")
			} else {
				r.openANSI(w, sgrBold)
			}
		} else if r.mode == ModeGotui {
			r.pushGotui("mod:italic")
		} else {
			r.openANSI(w, sgrItalic)
		}
	} else if r.mode == ModeGotui {
		r.popGotui()
	} else {
		r.closeANSI(w)
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderCodeSpan(
	w util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(source))
		}
	}
	if r.mode == ModeGotui {
		r.pushGotui("fg:cyan")
		r.writeStyled(w, b.String())
		r.popGotui()
	} else {
		r.openANSI(w, sgrCode)
		_, _ = w.WriteString(b.String())
		r.closeANSI(w)
	}
	return ast.WalkSkipChildren, nil
}

func (r *ansiRenderer) writeHighlightedCode(w util.BufWriter, source []byte, n ast.Node, lang string) {
	var b strings.Builder
	lines := n.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		b.Write((&line).Value(source))
	}
	body := b.String()
	label := strings.TrimSpace(lang)
	if label == "" {
		label = "code"
	}

	// Dim language header.
	r.writeToken(w, KindComment, label)
	_, _ = w.WriteString("\n")

	tokens := Highlight(lang, body)
	if len(tokens) == 0 {
		r.writeGutter(w)
		_, _ = w.WriteString("\n\n")
		return
	}

	// Emit tokens with a soft gutter at each line start.
	atLineStart := true
	for _, tok := range tokens {
		text := tok.Text
		for len(text) > 0 {
			if atLineStart {
				r.writeGutter(w)
				atLineStart = false
			}
			nl := strings.IndexByte(text, '\n')
			if nl < 0 {
				r.writeToken(w, tok.Kind, text)
				break
			}
			if nl > 0 {
				r.writeToken(w, tok.Kind, text[:nl])
			}
			_ = w.WriteByte('\n')
			atLineStart = true
			text = text[nl+1:]
		}
	}
	if !strings.HasSuffix(body, "\n") {
		_ = w.WriteByte('\n')
	}
	_, _ = w.WriteString("\n")
}

func (r *ansiRenderer) writeGutter(w util.BufWriter) {
	r.writeToken(w, KindPunct, "│ ")
}

func (r *ansiRenderer) writeToken(w util.BufWriter, kind TokenKind, text string) {
	if text == "" {
		return
	}
	if r.mode == ModeGotui {
		if style := gotuiStyleFor(kind); style != "" {
			r.pushGotui(style)
			r.writeStyled(w, text)
			r.popGotui()
			return
		}
		r.writeStyled(w, text)
		return
	}
	if sgr := ansiStyleFor(kind); sgr != "" {
		r.openANSI(w, sgr)
		_, _ = w.WriteString(text)
		r.closeANSI(w)
		return
	}
	_, _ = w.WriteString(text)
}

func ansiStyleFor(kind TokenKind) string {
	switch kind {
	case KindKeyword, KindTag:
		return sgrKeyword
	case KindString:
		return sgrString
	case KindComment:
		return sgrComment
	case KindAttr:
		return sgrAttr
	case KindNumber:
		return sgrNumber
	case KindPunct:
		return sgrPunct
	default:
		return ""
	}
}

func gotuiStyleFor(kind TokenKind) string {
	switch kind {
	case KindKeyword, KindTag:
		return "fg:cyan"
	case KindString:
		return "fg:green"
	case KindComment:
		return "fg:darkgrey,mod:italic"
	case KindAttr:
		return "fg:yellow"
	case KindNumber:
		return "fg:magenta"
	case KindPunct:
		return "fg:darkgrey"
	default:
		return ""
	}
}

func (r *ansiRenderer) renderCodeBlock(
	w util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		r.writeHighlightedCode(w, source, n, "")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderFencedCode(
	w util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if entering {
		lang := ""
		if fc, ok := n.(*ast.FencedCodeBlock); ok {
			lang = string(fc.Language(source))
		}
		r.writeHighlightedCode(w, source, n, lang)
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderList(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		r.listDepth++
		if l, ok := n.(*ast.List); ok && l.IsOrdered() {
			r.orderedIdx = append(r.orderedIdx, 0)
		} else {
			r.orderedIdx = append(r.orderedIdx, -1)
		}
	} else {
		r.listDepth--
		if len(r.orderedIdx) > 0 {
			r.orderedIdx = r.orderedIdx[:len(r.orderedIdx)-1]
		}
		_, _ = w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderListItem(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	indent := strings.Repeat("  ", maxInt(0, r.listDepth-1))
	_, _ = w.WriteString(indent)
	idx := -1
	if len(r.orderedIdx) > 0 {
		idx = r.orderedIdx[len(r.orderedIdx)-1]
	}
	if idx >= 0 {
		r.orderedIdx[len(r.orderedIdx)-1]++
		_, _ = w.WriteString(strconv.Itoa(r.orderedIdx[len(r.orderedIdx)-1]))
		_, _ = w.WriteString(". ")
	} else {
		_, _ = w.WriteString("• ")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderLink(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if r.mode == ModeGotui {
			r.pushGotui("fg:blue")
		} else {
			r.openANSI(w, sgrLink)
		}
	} else {
		if r.mode == ModeGotui {
			r.popGotui()
		} else {
			r.closeANSI(w)
		}
		link := n.(*ast.Link)
		_, _ = w.WriteString(" (")
		_, _ = w.Write(link.Destination)
		_ = w.WriteByte(')')
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderAutoLink(
	w util.BufWriter,
	source []byte,
	n ast.Node,
	entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	al := n.(*ast.AutoLink)
	url := string(al.URL(source))
	if r.mode == ModeGotui {
		r.pushGotui("fg:blue")
		r.writeStyled(w, url)
		r.popGotui()
	} else {
		r.openANSI(w, sgrLink)
		_, _ = w.WriteString(url)
		r.closeANSI(w)
	}
	return ast.WalkSkipChildren, nil
}

func (r *ansiRenderer) renderBlockquote(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("│ ")
		if r.mode == ModeGotui {
			r.pushGotui("fg:darkgrey,mod:italic")
		} else {
			r.openANSI(w, sgrQuote)
		}
	} else {
		if r.mode == ModeGotui {
			r.popGotui()
		} else {
			r.closeANSI(w)
		}
		_, _ = w.WriteString("\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderHR(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(strings.Repeat("─", minInt(r.width, 40)))
		_, _ = w.WriteString("\n\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderTable(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderTableRow(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("\n")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderTableCell(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("| ")
	} else {
		_, _ = w.WriteString(" ")
	}
	return ast.WalkContinue, nil
}

func (r *ansiRenderer) renderStrike(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if r.mode == ModeGotui {
			r.pushGotui("mod:strike")
		} else {
			r.openANSI(w, sgrStrike)
		}
	} else if r.mode == ModeGotui {
		r.popGotui()
	} else {
		r.closeANSI(w)
	}
	return ast.WalkContinue, nil
}

func wrapPlain(s string, width int) string {
	// Strip gotui markup for width calc by measuring visible text only.
	var b strings.Builder
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		wrapLinePlain(&b, line, width)
	}
	return b.String()
}

func wrapLinePlain(b *strings.Builder, line string, width int) {
	if visualMarkupWidth(line) <= width {
		b.WriteString(line)
		return
	}
	segs := splitGotuiSegments(line)
	var cur strings.Builder
	vis := 0
	lineStart := true
	emitBreak := func() {
		b.WriteString(cur.String())
		b.WriteByte('\n')
		cur.Reset()
		vis = 0
		lineStart = true
	}
	writeSeg := func(text, style string) {
		if text == "" {
			return
		}
		if style != "" {
			text = strings.ReplaceAll(text, "]", "〉")
			cur.WriteString("[")
			cur.WriteString(text)
			cur.WriteString("](")
			cur.WriteString(style)
			cur.WriteString(")")
		} else {
			cur.WriteString(text)
		}
		vis += runewidth.StringWidth(text)
		lineStart = false
	}
	for _, seg := range segs {
		remaining := seg.text
		for remaining != "" {
			rw := runewidth.StringWidth(remaining)
			if vis+rw <= width {
				writeSeg(remaining, seg.style)
				break
			}
			// Need to split remaining to fit.
			space := width - vis
			if space <= 0 && !lineStart {
				emitBreak()
				continue
			}
			if space <= 0 {
				space = width
			}
			cut := runewidth.Truncate(remaining, space, "")
			if cut == "" {
				// Single wide rune — force one char.
				_, size := utf8.DecodeRuneInString(remaining)
				cut = remaining[:size]
			}
			// Prefer breaking at last space in the cut when unstyled or whole styled chunk.
			if sp := strings.LastIndexByte(cut, ' '); sp > 0 && runewidth.StringWidth(cut[:sp]) >= space/2 {
				cut = cut[:sp]
			}
			writeSeg(cut, seg.style)
			remaining = strings.TrimLeft(remaining[len(cut):], " ")
			if remaining != "" {
				emitBreak()
			}
		}
	}
	b.WriteString(cur.String())
}

type gotuiSeg struct {
	text  string
	style string // empty = unstyled
}

func splitGotuiSegments(s string) []gotuiSeg {
	var out []gotuiSeg
	runes := []rune(s)
	var plain strings.Builder
	flushPlain := func() {
		if plain.Len() == 0 {
			return
		}
		out = append(out, gotuiSeg{text: plain.String()})
		plain.Reset()
	}
	for i := 0; i < len(runes); {
		if runes[i] == '[' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == ']' {
					end = j
					break
				}
			}
			if end > 0 && end+1 < len(runes) && runes[end+1] == '(' {
				closeParen := -1
				for j := end + 2; j < len(runes); j++ {
					if runes[j] == ')' {
						closeParen = j
						break
					}
				}
				if closeParen > 0 {
					flushPlain()
					text := string(runes[i+1 : end])
					style := string(runes[end+2 : closeParen])
					out = append(out, gotuiSeg{text: text, style: style})
					i = closeParen + 1
					continue
				}
			}
		}
		plain.WriteRune(runes[i])
		i++
	}
	flushPlain()
	return out
}

func stripGotuiMarkup(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if runes[i] == '[' {
			end := -1
			for j := i + 1; j < len(runes); j++ {
				if runes[j] == ']' {
					end = j
					break
				}
			}
			if end > 0 && end+1 < len(runes) && runes[end+1] == '(' {
				closeParen := -1
				for j := end + 2; j < len(runes); j++ {
					if runes[j] == ')' {
						closeParen = j
						break
					}
				}
				if closeParen > 0 {
					b.WriteString(string(runes[i+1 : end]))
					i = closeParen + 1
					continue
				}
			}
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String()
}

func visualMarkupWidth(s string) int {
	return runewidth.StringWidth(stripGotuiMarkup(s))
}

func wrapANSI(s string, width int) string {
	if width <= 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + len(s)/width)
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		wrapLineANSI(&b, line, width)
	}
	return b.String()
}

func wrapLineANSI(b *strings.Builder, line string, width int) {
	if visualWidth(line) <= width {
		b.WriteString(line)
		return
	}
	var cur strings.Builder
	vis := 0
	inESC := false
	for _, r := range line {
		if r == '\x1b' {
			inESC = true
			cur.WriteRune(r)
			continue
		}
		if inESC {
			cur.WriteRune(r)
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inESC = false
			}
			continue
		}
		rw := runewidth.RuneWidth(r)
		if vis+rw > width && vis > 0 {
			seg := cur.String()
			if sp := lastSpaceOutsideANSI(seg); sp >= 0 {
				b.WriteString(strings.TrimRightFunc(seg[:sp], unicode.IsSpace))
				b.WriteByte('\n')
				rest := strings.TrimLeftFunc(seg[sp:], unicode.IsSpace)
				cur.Reset()
				cur.WriteString(rest)
				vis = visualWidth(rest)
			} else {
				b.WriteString(seg)
				b.WriteByte('\n')
				cur.Reset()
				vis = 0
			}
		}
		cur.WriteRune(r)
		vis += rw
	}
	b.WriteString(cur.String())
}

func lastSpaceOutsideANSI(s string) int {
	inESC := false
	last := -1
	for i, r := range s {
		if r == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inESC = false
			}
			continue
		}
		if unicode.IsSpace(r) {
			last = i
		}
	}
	return last
}

func visualWidth(s string) int {
	return runewidth.StringWidth(stripANSI(s))
}

func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inESC := false
	for _, r := range s {
		if r == '\x1b' {
			inESC = true
			continue
		}
		if inESC {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inESC = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func runewidthString(s string) int {
	return runewidth.StringWidth(s)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
