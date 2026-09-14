package cli

import (
	"io"

	"github.com/fatih/color"
)

// Shared color helpers. fatih/color disables ANSI codes automatically when
// the output is not a terminal, so piped output stays machine-readable.
var (
	colGreen  = color.New(color.FgGreen)
	colCyan   = color.New(color.FgCyan)
	colYellow = color.New(color.FgYellow)
	colRed    = color.New(color.FgRed)
)

func green(w io.Writer, format string, args ...any) {
	colGreen.Fprintf(w, format, args...)
}

func cyan(w io.Writer, format string, args ...any) {
	colCyan.Fprintf(w, format, args...)
}

func yellow(w io.Writer, format string, args ...any) {
	colYellow.Fprintf(w, format, args...)
}

func red(w io.Writer, format string, args ...any) {
	colRed.Fprintf(w, format, args...)
}
