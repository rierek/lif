package commands

import (
	"testing"

	"github.com/rierek/lif/pkg/config"
)

func TestResolveWidthPrefersTheFlag(t *testing.T) {
	cfg := &config.Config{Display: config.DisplayConfig{MaxWidth: 140}}
	if got := resolveWidth(90, true, cfg); got != 90 {
		t.Errorf("resolveWidth = %d, want the flag's 90", got)
	}
}

func TestResolveWidthFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{Display: config.DisplayConfig{MaxWidth: 140}}
	got := resolveWidth(0, false, cfg)
	if got > 140 {
		t.Errorf("resolveWidth = %d, want at most the configured 140", got)
	}
}

func TestResolveWidthFloorsAt80(t *testing.T) {
	cfg := &config.Config{}
	if got := resolveWidth(40, true, cfg); got != 80 {
		t.Errorf("resolveWidth = %d, want the floor of 80", got)
	}
}

func TestResolveWidthZeroDisablesTheCap(t *testing.T) {
	cfg := &config.Config{}
	got := resolveWidth(0, true, cfg)
	if got < 80 {
		t.Errorf("uncapped width = %d, want at least the terminal width or 80", got)
	}
}

// TestResolveWidthNotChangedFallsBackToConfigUncapped covers the fourth case
// the brief calls out explicitly: the config's own -1 (uncapped) must also
// be honoured when the flag was not given at all -- distinct from the flag
// being given as a literal 0, which is covered above via flagChanged=true.
func TestResolveWidthNotChangedFallsBackToConfigUncapped(t *testing.T) {
	cfg := &config.Config{Display: config.DisplayConfig{MaxWidth: -1}}
	got := resolveWidth(0, false, cfg)
	if got < 80 {
		t.Errorf("resolveWidth = %d, want at least the floor of 80 (config -1 = uncapped)", got)
	}
}

// TestResolveWidthDistinguishesFlagZeroFromUnchanged is the case the brief
// warns is easy to get wrong: flagValue==0 means two different things
// depending on flagChanged, and cobra's own zero value must not be
// mistaken for the user explicitly typing --width 0.
func TestResolveWidthDistinguishesFlagZeroFromUnchanged(t *testing.T) {
	cfg := &config.Config{Display: config.DisplayConfig{MaxWidth: 55}}

	// Flag not given at all: cobra leaves flagValue at its zero value (0),
	// but flagChanged is false, so the config's cap (floored to 80) applies.
	notGiven := resolveWidth(0, false, cfg)
	if notGiven != 80 {
		t.Errorf("resolveWidth(not given) = %d, want the config's 55 floored to 80", notGiven)
	}

	// Flag explicitly given as 0: flagChanged is true, which must disable
	// the cap entirely rather than being floored to 80 like the config's 55.
	givenZero := resolveWidth(0, true, cfg)
	if givenZero < 80 {
		t.Errorf("resolveWidth(explicit 0) = %d, want at least the floor of 80 (uncapped)", givenZero)
	}
	if givenZero == notGiven {
		t.Errorf("resolveWidth(explicit 0) = %d, want it to differ from the not-given case (%d): "+
			"an explicit 0 disables the cap, an unset flag falls back to the config's small cap", givenZero, notGiven)
	}
}
