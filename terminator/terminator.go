package terminator

import (
	"bytes"
	"io"
	"math"
	"strings"
	"sync"
)

type Color string

const (
	Reset   Color = "\033[0m"
	Red     Color = "\033[31m"
	Orange  Color = "\033[38;5;208m"
	Yellow  Color = "\033[33m"
	Green   Color = "\033[32m"
	Cyan    Color = "\033[36m"
	Blue    Color = "\033[34m"
	Magenta Color = "\033[35m"
)

const (
	up    = "\033[1A"
	down    = "\033[1B"
	clear = "\033[2K"
	clearToEnd = "\033[0K"
	save = "\033[s"
	restore = "\033[u"
)

type Line interface {
	Next() []byte
	GetOptions() *Options
}

type Options struct {
	WithNextLine bool
	ManualCleanup func() []byte
}

type line struct {
	next func() []byte
	*Options
}

func (l *line) Next() []byte {
	return l.next()
}

func (l *line) GetOptions() *Options {
	return l.Options
}

func LineFromGenerator(g func() []byte) Line {
	return &line {
		next: g,
		Options: &Options {
			WithNextLine: true,
		},
	}
}

func InLineFromGenerator(g func() []byte) Line {
	return &line {
		next: g,
		Options: &Options {
			WithNextLine: false,
		},
	}
}

func LineFromGeneratorAndOptions(g func() []byte, o *Options) Line {
	return &line {
		next: g,
		Options: o,
	}
}

func AnimatedLineFromGenerator(g func() []byte, colors []Color) Line {
	shift := 0

	return LineFromGenerator(func ()[]byte {
		msg := g()

		if msg == nil {
			return nil
		}

		var buffer bytes.Buffer

		buffer.WriteString(string(colors[int(math.Abs(float64(shift-1)))%len(colors)]))
		for i, c := range msg {
			buffer.WriteByte(c)
			buffer.WriteString(string(colors[(i+shift)%len(colors)]))
		}

		buffer.WriteString(string(Reset))

		shift++
		return buffer.Bytes()
	})
}

func Spacer() Line {
	return &line {
		next: func() []byte { return []byte("<spacer>") },
		Options: &Options{
			WithNextLine: true,
		},
	}
}

// --

type Terminator interface {
	Draw() error
	HadInput()
	AddLine(Line)
	RemoveLine(Line)
	CreateInputLine(rune) Line
	HideLine(Line)
	ShowLine(Line)
}

type terminator struct {
	io.Writer	
	sync.Mutex
	lines []Line
	hiddenLines []bool
	drawClearBalance []int
	cursor int
	offTheBottom int
	needToClearInput bool
	hasSavedPosition bool
}

func New(w io.Writer) Terminator {
	return &terminator{
		Writer: w,
	}
}

func (t *terminator) CreateInputLine(symbol rune) Line {
	inputLine := LineFromGeneratorAndOptions(func() []byte {
		var buf bytes.Buffer

		buf.WriteString(down)

		buf.WriteRune(symbol)
		buf.WriteRune(' ')

		if t.needToClearInput {
			buf.WriteString(clearToEnd)
			t.needToClearInput = false
			t.hasSavedPosition = false
		}
		if t.hasSavedPosition {
			buf.WriteString(restore)
		}

		return buf.Bytes()
	}, &Options{
		WithNextLine: false,
		ManualCleanup: func() []byte {
			var buf bytes.Buffer

			buf.WriteString(save)
			t.hasSavedPosition = true

			buf.WriteString(down)
			buf.WriteString("\r")
			buf.WriteString(clear)
			buf.WriteString(up)

			buf.WriteString(up)

			return buf.Bytes()
		},
	})

	return inputLine
}

func (t *terminator) draw() error {
	for i, line := range t.lines {
		if t.hiddenLines != nil && t.hiddenLines[i] {
			continue
		}

		opts := line.GetOptions()

		next := line.Next()

		t.Write([]byte(clear))

		if next != nil && string(next) != "<spacer>" {
			if _, err := t.Write(next); err != nil {
				return err
			}
			t.drawClearBalance[i]++
		}

		if opts.WithNextLine {
			t.Write([]byte("\n"))
			t.cursor++
		}
	}

	return nil
}

func (t *terminator) clear() {
	if t.cursor <= 0 {
		t.cursor = 0
		t.offTheBottom = 0
		return
	}

	t.Write([]byte(strings.Repeat("\r" + clear + up, t.offTheBottom)))
	t.offTheBottom = 0
	t.cursor -= t.offTheBottom

	for i := len(t.lines) - 1; i > -1; i-- {
		if  t.drawClearBalance != nil && t.drawClearBalance[i] <= 0 {
			continue
		}

		l := t.lines[i]
		opts := l.GetOptions()

		if opts.ManualCleanup != nil {
			t.Write(opts.ManualCleanup())
			t.drawClearBalance[i]--
			continue
		}

		if opts.WithNextLine {
			t.Write([]byte("\r" + clear + up))
			t.cursor--
			t.drawClearBalance[i]--
		} 
	}
}

func (t *terminator) Draw() error {
	t.Lock()
	defer t.Unlock()

	t.clear()
	return t.draw()
}

func (t *terminator) HadInput() {
	t.Lock()
	defer t.Unlock()

	t.cursor++
	t.offTheBottom++
	t.needToClearInput = true
}

func (t *terminator) AddLine(l Line) {
	t.Lock()
	defer t.Unlock()

	t.lines = append(t.lines, l)
	t.hiddenLines = append(t.hiddenLines, false)
	t.drawClearBalance = append(t.drawClearBalance, 0)
}

func (t *terminator) RemoveLine(l Line) {
	t.Lock()
	defer t.Unlock()

	if len(t.lines) <= 1 {
		t.lines = nil
		t.hiddenLines = nil
		t.drawClearBalance = nil
		return
	}

	i := t.findLine(l)

	if i < 0 {
		return
	}

	t.lines = append(t.lines[:i], t.lines[i+1:]...)
	t.hiddenLines = append(t.hiddenLines[:i], t.hiddenLines[i+1:]...)
	t.drawClearBalance = append(t.drawClearBalance[:i], t.drawClearBalance[i+1:]...)
}

func (t *terminator) ShowLine(l Line) {
	t.Lock()
	defer t.Unlock()

	if t.hiddenLines == nil {
		return
	}

	i := t.findLine(l)

	if i < 0 {
		return
	}

	t.hiddenLines[i] = false
}

func (t *terminator) HideLine(l Line) {
	t.Lock()
	defer t.Unlock()

	i := t.findLine(l)

	if i < 0 {
		return
	}

	t.hiddenLines[i] = true
}

func (t *terminator) findLine(l Line) int {
	for i := 0; i < len(t.lines); i++ {
		if t.lines[i] == l {
			return i
		}
	}

	return -1
}