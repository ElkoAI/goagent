package reporting

import (
	"strings"
	"testing"
)

func TestGetSymbolMeta_Known(t *testing.T) {
	m := GetSymbolMeta("AAPL")
	if m.LongName != "Apple Inc." {
		t.Errorf("AAPL LongName = %q, want %q", m.LongName, "Apple Inc.")
	}
	if m.Sector != "US Mag 7" {
		t.Errorf("AAPL Sector = %q, want %q", m.Sector, "US Mag 7")
	}
	if m.Type != "stock" {
		t.Errorf("AAPL Type = %q, want %q", m.Type, "stock")
	}
}

func TestGetSymbolMeta_Unknown(t *testing.T) {
	m := GetSymbolMeta("ZZZZ")
	if m.Symbol != "ZZZZ" {
		t.Errorf("Symbol = %q, want %q", m.Symbol, "ZZZZ")
	}
	if m.ShortName != "ZZZZ" {
		t.Errorf("ShortName = %q, want %q", m.ShortName, "ZZZZ")
	}
	if m.LongName != "" {
		t.Errorf("LongName = %q, want empty", m.LongName)
	}
}

func TestLogoSVG_KnownSymbol(t *testing.T) {
	svg := LogoSVG("AAPL")
	s := string(svg)
	if !strings.Contains(s, `class="logo"`) {
		t.Error("expected logo wrapper class")
	}
	if !strings.Contains(s, "<svg") {
		t.Error("expected inline SVG for known symbol")
	}
}

func TestLogoSVG_UnknownSymbol(t *testing.T) {
	svg := LogoSVG("ZZZZ")
	s := string(svg)
	if !strings.Contains(s, `class="logo-badge"`) {
		t.Error("expected fallback badge class")
	}
	if !strings.Contains(s, "ZZ") {
		t.Error("expected first 2 letters in badge")
	}
}

func TestLogoSVG_ShortSymbol(t *testing.T) {
	svg := LogoSVG("X")
	s := string(svg)
	if !strings.Contains(s, "X") {
		t.Error("expected single letter in badge")
	}
}

func TestLoadLogos_AllSymbolsHaveLogos(t *testing.T) {
	logos := loadLogos()
	for sym := range symbolRegistry {
		if _, ok := logos[sym]; !ok {
			t.Errorf("missing logo SVG for registered symbol %s", sym)
		}
	}
}

func TestLoadLogos_SVGContent(t *testing.T) {
	logos := loadLogos()
	for sym, svg := range logos {
		if !strings.Contains(svg, "<svg") {
			t.Errorf("logo for %s does not contain <svg tag", sym)
		}
	}
}
