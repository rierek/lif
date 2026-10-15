package commands

import (
	"os"

	"golang.org/x/term"

	"github.com/rierek/lif/pkg/config"
)

const (
	widthFloor   = 80
	widthDefault = config.DefaultMaxWidth
)

// resolveWidth decides the rendering width: the flag wins, then the config,
// then 110. A cap of 0 disables capping and yields the raw terminal width
// (or the default, when that can't be detected — e.g. output is piped, or a
// test harness with no controlling tty). The floor of 80 is absolute — below
// it no layout is readable.
//
// flagChanged is what makes case 1 ("--width not given") distinguishable
// from case 2 ("--width 0 given explicitly"): cobra reports flagValue as 0
// in both, and only flagChanged tells them apart.
//
// A terminal narrower than 80 columns is not treated as "the real width" for
// clamping purposes when no cap applies — an undetectable terminal must not
// silently collapse every render to the floor, which is what naively
// defaulting the detected width to widthFloor would do.
func resolveWidth(flagValue int, flagChanged bool, cfg *config.Config) int {
	widthCap := widthDefault
	switch {
	case flagChanged:
		widthCap = flagValue
	case cfg != nil:
		widthCap = cfg.GetMaxWidth()
	}

	uncapped := widthCap <= 0 // 0 from the flag, or -1 from the config
	target := widthCap
	if uncapped {
		target = widthDefault
	}

	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		if uncapped || w < target {
			target = w
		}
	}

	if target < widthFloor {
		target = widthFloor
	}
	return target
}
