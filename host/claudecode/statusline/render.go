package statusline

import (
	"strings"

	"github.com/muesli/termenv"
)

// Segment is one styled chunk of statusline output. A consumer builds the
// whole line as a slice of Segments — including any literal separators
// (e.g. " | ") as their own plain Segment — since Render performs no
// implicit joining beyond concatenation.
//
// Color is optional and consumer-agnostic: a hex string ("#ff0000") or a
// termenv ANSI color-code string ("0"-"255"), resolved through termenv's
// profile-aware degradation at render time. Leaving it empty (pogopin's
// plain segments) renders unstyled text; ouroboros's colored segments (its
// existing priorityColor semantics) populate it. Dim/Bold apply independent
// of Color.
type Segment struct {
	Text  string
	Color string
	Dim   bool
	Bold  bool
}

// RenderOptions controls Render's color-profile selection.
type RenderOptions struct {
	// Plain forces every segment to render as bare text, regardless of
	// Profile or the environment — set this from an explicit --plain flag
	// or a NO_COLOR check.
	Plain bool

	// Profile overrides the auto-detected termenv color profile. Nil
	// selects termenv.EnvColorProfile() (which itself already degrades to
	// termenv.Ascii when NO_COLOR is set). Tests force a specific tier
	// (TrueColor/ANSI256/ANSI/Ascii) via this field to exercise
	// degradation deterministically, independent of the process
	// environment/TTY.
	Profile *termenv.Profile
}

// Render concatenates segments into one line, applying each segment's
// Color/Dim/Bold through termenv's profile-aware color degradation
// (TrueColor -> ANSI256 -> ANSI -> Ascii). Ascii (Plain, NO_COLOR, or a
// dumb terminal) collapses every segment to its bare Text with no escape
// sequences at all — the same code path a plain-segment consumer
// (pogopin) and a colored-segment consumer (ouroboros) both render
// through; only what each populates on Segment differs.
func Render(segments []Segment, opts RenderOptions) string {
	profile := termenv.EnvColorProfile()
	if opts.Profile != nil {
		profile = *opts.Profile
	}

	if opts.Plain {
		profile = termenv.Ascii
	}

	var sb strings.Builder

	for _, seg := range segments {
		sb.WriteString(styleSegment(seg, profile))
	}

	return sb.String()
}

// styleSegment renders one Segment under profile, degrading Color (if any)
// through profile.Convert and short-circuiting to bare text when profile
// is termenv.Ascii (termenv.Style.Styled's own behavior).
func styleSegment(seg Segment, profile termenv.Profile) string {
	style := profile.String(seg.Text)

	if seg.Color != "" {
		style = style.Foreground(profile.Color(seg.Color))
	}
	if seg.Dim {
		style = style.Faint()
	}
	if seg.Bold {
		style = style.Bold()
	}

	return style.String()
}
