package waves

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/rierek/lif/internal/tui/theme"
)

// RenderOpts is the view state Render needs on top of the Board.
type RenderOpts struct {
	// Width and Height are the cells of the output cut.
	Width, Height int
	// Focus is the focused card; its edges are drawn heavy.
	Focus string
	// Mark is the card marked as predecessor; empty means no mark.
	Mark string
	// Dimmed holds the cards that are not selectable.
	Dimmed map[string]bool
	// Critical switches the critical-path highlight on.
	Critical bool
}

// Render paints the board and returns the cut that follows the focus: the
// focus card stands in the second visible display column, the loose column and
// column 0 stand at the left edge without an empty column before them, and
// the rows of the focus card are fully visible. It returns at most Height
// lines of at most Width cells. A marker sits on every edge of the cut with
// cards beyond it.
func (b *Board) Render(o RenderOpts) string {
	if len(b.byRank) == 0 || o.Width <= 0 || o.Height <= 0 {
		return ""
	}
	cv := b.paint(o)
	ox, oy := 0, 0
	if f, ok := b.cards[o.Focus]; ok {
		x := b.colX[f.col]
		ox = min(x, max(b.colX[max(f.col-1, 0)], x+cardW-o.Width))
		oy = min(max(y0(f.row)+cardH/2-o.Height/2, 0), max(cv.h-o.Height, 0))
	}
	w, h := min(o.Width, cv.w-ox), min(o.Height, cv.h-oy)

	var left, right, up, down bool
	for _, c := range b.byRank {
		x, y := b.colX[c.col], y0(c.row)
		left, right = left || x < ox, right || x+cardW > ox+o.Width
		up, down = up || y < oy, down || y+cardH > oy+o.Height
	}
	mark := sty{fg: theme.Peach}
	put := func(x, y int, r rune) {
		if c := cv.at(ox+x, oy+y); c != nil {
			*c = cell{r: r, st: mark}
		}
	}
	if left {
		put(0, h/2, '◀')
	}
	if right {
		put(w-1, h/2, '▶')
	}
	if up {
		put(w/2, 0, '▲')
	}
	if down {
		put(w/2, h-1, '▼')
	}
	return cv.view(ox, oy, w, h)
}

// paint draws edges, the loose-column separator and cards for one view state.
func (b *Board) paint(o RenderOpts) *canvas {
	last := len(b.Columns) - 1
	cv := newCanvas(b.colX[last]+cardW, b.height())
	for i, e := range b.edges {
		if !b.drawn(i) {
			continue
		}
		lvl, heavy := lvlDirect, e.From == o.Focus || e.To == o.Focus
		switch {
		case o.Critical && b.critEdge[i]:
			lvl = lvlCritical
		case heavy:
			lvl = lvlFocus
		case e.Lifted:
			lvl = lvlLifted
		}
		for _, s := range b.segments(i) {
			cv.line(s, lvl, heavy)
		}
		x, y := b.arrow(i)
		cv.at(x, y).arrow = true
	}
	if b.Loose && last > 0 {
		x := b.colX[0] + cardW + (b.colX[1]-b.colX[0]-cardW)/2 - 1
		for y := range cv.h {
			*cv.at(x, y) = cell{r: '┊', st: sty{fg: theme.Surface}}
		}
	}
	for _, c := range b.byRank {
		cv.card(c, b.colX[c.col], o, b.critCard[c.id])
	}
	return cv
}

// Line importance at a crossing, lowest first. It also picks the colour.
const (
	lvlLifted uint8 = iota + 1
	lvlDirect
	lvlFocus
	lvlCritical
)

func lvlStyle(l uint8) sty {
	switch l {
	case lvlLifted:
		return sty{fg: theme.Overlay}
	case lvlFocus:
		return sty{fg: theme.Mauve}
	case lvlCritical:
		return sty{fg: theme.Red}
	}
	return sty{fg: theme.Text}
}

// sty is the style of one cell. The zero value is unstyled.
type sty struct {
	fg   lipgloss.Color
	bold bool
}

func (s sty) render(text string) string {
	if s == (sty{}) {
		return text
	}
	st := lipgloss.NewStyle().Foreground(s.fg)
	if s.bold {
		st = st.Bold(true)
	}
	return st.Render(text)
}

// Arms of a line cell.
const (
	armU uint8 = 1 << iota
	armD
	armL
	armR
)

// stroke is the lines of one axis in a cell: arms, the arms that are heavy and
// the importance of the most important line.
type stroke struct{ mask, heavy, lvl uint8 }

func (s *stroke) add(arms, lvl uint8, heavy bool) {
	s.mask |= arms
	s.lvl = max(s.lvl, lvl)
	if heavy {
		s.heavy |= arms
	}
}

// cell is one canvas cell: text, or horizontal and vertical line strokes. turn
// marks a cell where an edge bends, so both axes belong together; without turn
// two axes in one cell are a crossing.
type cell struct {
	r    rune
	st   sty
	cont bool // second half of a wide rune
	h, v stroke
	turn bool
	// arrow marks the arrowhead cell in front of a target card; it takes the
	// importance of the lines that end there.
	arrow bool
}

func (c *cell) crossing() bool {
	return c.h.mask != 0 && c.v.mask != 0 && (!c.turn || c.h.mask|c.v.mask == armU|armD|armL|armR)
}

type canvas struct {
	w, h  int
	cells [][]cell
}

func newCanvas(w, h int) *canvas {
	cv := &canvas{w: w, h: h, cells: make([][]cell, h)}
	for y := range cv.cells {
		cv.cells[y] = make([]cell, w)
	}
	return cv
}

func (cv *canvas) at(x, y int) *cell {
	if x < 0 || y < 0 || x >= cv.w || y >= cv.h {
		return nil
	}
	return &cv.cells[y][x]
}

// line draws one segment of an edge. The ends of a vertical segment are turns.
func (cv *canvas) line(s segment, lvl uint8, heavy bool) {
	if s.vertical() {
		a, z := min(s.y1, s.y2), max(s.y1, s.y2)
		for y := a; y <= z; y++ {
			var arms uint8
			if y > a {
				arms |= armU
			}
			if y < z {
				arms |= armD
			}
			if c := cv.at(s.x1, y); c != nil {
				c.v.add(arms, lvl, heavy)
				c.turn = c.turn || y == a || y == z
			}
		}
		return
	}
	a, z := min(s.x1, s.x2), max(s.x1, s.x2)
	for x := a; x <= z; x++ {
		var arms uint8
		if x > a {
			arms |= armL
		}
		if x < z {
			arms |= armR
		}
		if c := cv.at(x, s.y1); c != nil {
			c.h.add(arms, lvl, heavy)
		}
	}
}

// glyph returns the line rune of a cell and its importance. A bend draws both
// axes together; a crossing draws only the more important axis, the other
// line is interrupted. The output never contains a four-way junction.
func (c *cell) glyph() (rune, uint8) {
	straight := func(horizontal bool, s stroke) rune {
		heavy := s.heavy != 0
		switch {
		case horizontal && heavy:
			return '━'
		case horizontal:
			return '─'
		case heavy:
			return '┃'
		}
		return '│'
	}
	switch {
	case c.arrow:
		return '▶', c.h.lvl
	case c.h.mask == 0:
		return straight(false, c.v), c.v.lvl
	case c.v.mask == 0:
		return straight(true, c.h), c.h.lvl
	case c.crossing():
		if c.v.lvl > c.h.lvl {
			return straight(false, c.v), c.v.lvl
		}
		return straight(true, c.h), c.h.lvl
	}
	return junction(c.h.mask|c.v.mask, c.h.heavy|c.v.heavy), max(c.h.lvl, c.v.lvl)
}

// junction picks the rune of a bend or a T for the arms and the heavy arms.
// Every arm keeps the weight of its own edge, so a branch can mix weights.
func junction(arms, heavy uint8) rune {
	heavy &= arms
	var table map[uint8]rune
	switch arms {
	case armD | armR:
		table = map[uint8]rune{0: '╭', armR: '┍', armD: '┎', armD | armR: '┏'}
	case armD | armL:
		table = map[uint8]rune{0: '╮', armL: '┑', armD: '┒', armD | armL: '┓'}
	case armU | armR:
		table = map[uint8]rune{0: '╰', armR: '┕', armU: '┖', armU | armR: '┗'}
	case armU | armL:
		table = map[uint8]rune{0: '╯', armL: '┙', armU: '┚', armU | armL: '┛'}
	case armU | armD | armR:
		table = map[uint8]rune{0: '├', armR: '┝', armU: '┞', armD: '┟', armU | armD: '┠',
			armU | armR: '┡', armD | armR: '┢', armU | armD | armR: '┣'}
	case armU | armD | armL:
		table = map[uint8]rune{0: '┤', armL: '┥', armU: '┦', armD: '┧', armU | armD: '┨',
			armU | armL: '┩', armD | armL: '┪', armU | armD | armL: '┫'}
	case armL | armR | armD:
		table = map[uint8]rune{0: '┬', armL: '┭', armR: '┮', armL | armR: '┯', armD: '┰',
			armL | armD: '┱', armR | armD: '┲', armL | armR | armD: '┳'}
	case armL | armR | armU:
		table = map[uint8]rune{0: '┴', armL: '┵', armR: '┶', armL | armR: '┷', armU: '┸',
			armL | armU: '┹', armR | armU: '┺', armL | armR | armU: '┻'}
	}
	return table[heavy]
}

// put writes text at (x, y) and returns the next free x. Wide runes take two
// cells; runes without width are dropped.
func (cv *canvas) put(x, y int, s string, st sty) int {
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			continue
		}
		if c := cv.at(x, y); c != nil {
			*c = cell{r: r, st: st}
		}
		if w == 2 {
			if c := cv.at(x+1, y); c != nil {
				*c = cell{cont: true}
			}
		}
		x += w
	}
	return x
}

// card draws one card: top border with status dot and type, up to three title
// rows, bottom border with ID and the hints about edges outside the root.
func (cv *canvas) card(c *card, x int, o RenderOpts, critical bool) {
	y := y0(c.row)
	muted := c.it.Status == "completed" || o.Dimmed[c.id]
	frame, text, sub := sty{fg: theme.Overlay}, sty{fg: theme.Text}, sty{fg: theme.Subtext}
	if muted {
		frame, text, sub = sty{fg: theme.Surface}, sty{fg: theme.Overlay}, sty{fg: theme.Surface}
	}
	heavy := false
	switch c.id {
	case o.Mark:
		frame, heavy = sty{fg: theme.Teal}, true
	case o.Focus:
		frame, heavy = sty{fg: theme.Mauve}, true
	}
	tl, tr, bl, br, hz, vt := '╭', '╮', '╰', '╯', '─', '│'
	if heavy {
		tl, tr, bl, br, hz, vt = '┏', '┓', '┗', '┛', '━', '┃'
	}
	for dy := range cardH {
		for dx := range cardW {
			r, st := ' ', sty{}
			switch {
			case dy == 0 && dx == 0:
				r, st = tl, frame
			case dy == 0 && dx == cardW-1:
				r, st = tr, frame
			case dy == cardH-1 && dx == 0:
				r, st = bl, frame
			case dy == cardH-1 && dx == cardW-1:
				r, st = br, frame
			case dy == 0 || dy == cardH-1:
				r, st = hz, frame
			case dx == 0 || dx == cardW-1:
				r, st = vt, frame
			}
			*cv.at(x+dx, y+dy) = cell{r: r, st: st}
		}
	}

	idSt := text
	switch {
	case critical && o.Critical && c.id == o.Focus:
		idSt = sty{fg: theme.Red, bold: true}
	case critical && o.Critical:
		idSt = sty{fg: theme.Red}
	case c.id == o.Focus:
		idSt = sty{fg: theme.Mauve, bold: true}
	}

	// Inside the frame: x+2 .. x+cardW-3.
	const inner = cardW - 4
	cv.put(x+2, y, " ", frame)
	cv.put(x+3, y, "●", sty{fg: theme.StatusColor(c.it.Status)})
	cv.put(x+4, y, " ", frame)
	if t := fit(c.it.Type, 14); t != "" {
		cv.put(x+cardW-3-runewidth.StringWidth(t)-1, y, " "+t+" ", sub)
	}
	hint := ""
	if c.extIn > 0 {
		hint += "◀" + strconv.Itoa(c.extIn)
	}
	if c.extOut > 0 {
		hint += "▶" + strconv.Itoa(c.extOut)
	}
	room := inner - 2
	if hint != "" {
		hw := runewidth.StringWidth(hint)
		cv.put(x+cardW-3-hw-1, y+cardH-1, " "+hint+" ", sty{fg: theme.Overlay})
		room -= hw + 2
	}
	cv.put(x+2, y+cardH-1, " "+fit(c.id, room)+" ", idSt)
	for i, l := range wrap(c.it.Title, inner, cardH-2) {
		cv.put(x+2, y+1+i, l, text)
	}
}

// width is the cell width of s without control and zero-width runes.
func width(s string) int { return runewidth.StringWidth(clean(s)) }

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || runewidth.RuneWidth(r) == 0 {
			return -1
		}
		return r
	}, s)
}

// cut splits s after at most w cells.
func cut(s string, w int) (head, rest string) {
	n := 0
	for i, r := range s {
		rw := runewidth.RuneWidth(r)
		if n+rw > w {
			return s[:i], s[i:]
		}
		n += rw
	}
	return s, ""
}

// fit shortens s to w cells and ends it with an ellipsis when it was cut.
func fit(s string, w int) string {
	s = clean(s)
	if width(s) <= w {
		return s
	}
	head, _ := cut(s, max(w-1, 0))
	return head + "…"
}

// wrap breaks a title into at most lines rows of w cells. Words that are
// longer than a row break hard; a title that needs more rows is cut after the
// last row with an ellipsis.
func wrap(title string, w, lines int) []string {
	var out []string
	cur := ""
	for _, word := range strings.Fields(clean(title)) {
		for word != "" {
			switch {
			case cur == "" && width(word) <= w:
				cur, word = word, ""
			case cur != "" && width(cur)+1+width(word) <= w:
				cur, word = cur+" "+word, ""
			case cur != "":
				out, cur = append(out, cur), ""
			default:
				var head string
				head, word = cut(word, w)
				out = append(out, head)
			}
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	if len(out) > lines {
		out = out[:lines]
		out[lines-1] = fit(out[lines-1]+"…", w)
	}
	return out
}

// view renders the cut [ox, ox+w) x [oy, oy+h) as styled lines.
func (cv *canvas) view(ox, oy, w, h int) string {
	var sb strings.Builder
	for y := oy; y < oy+h; y++ {
		var run strings.Builder
		cur := sty{}
		flush := func() {
			sb.WriteString(cur.render(run.String()))
			run.Reset()
		}
		for x := ox; x < ox+w; x++ {
			c := cv.at(x, y)
			r, st := ' ', sty{}
			switch {
			case c.cont:
				continue
			case c.r != 0:
				r, st = c.r, c.st
			case c.arrow || c.h.mask|c.v.mask != 0:
				var lvl uint8
				r, lvl = c.glyph()
				st = lvlStyle(lvl)
			}
			if st != cur {
				flush()
				cur = st
			}
			run.WriteRune(r)
		}
		flush()
		if y < oy+h-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
