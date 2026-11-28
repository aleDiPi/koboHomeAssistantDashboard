package config

import (
	"strings"
	"testing"
)

func TestExampleFileParses(t *testing.T) {
	c, err := Load("../../dashboard.example.ini")
	if err != nil {
		t.Fatal(err)
	}
	if c.Orientation != "landscape" || len(c.Lights) != 2 || c.Token != "" {
		t.Errorf("unexpected: %+v", c)
	}
}

// Windows editors may save a byte order mark and CRLF line endings.
func TestByteOrderMark(t *testing.T) {
	file := string([]rune{0xFEFF}) + strings.Join([]string{"[lights]", "light.a = A", ""}, string([]rune{13, 10}))
	c, err := Parse(strings.NewReader(file))
	if err != nil || len(c.Lights) != 1 || c.Lights[0].Name != "A" {
		t.Fatalf("%+v %v", c.Lights, err)
	}
}

func TestIconAndWindows(t *testing.T) {
	c, err := Parse(strings.NewReader("[lights]" + string(rune(10)) + "light.a = Sofa | mdi:sofa" + string(rune(10)) +
		"[windows]" + string(rune(10)) + "cover.roof = Roof"))
	if err != nil {
		t.Fatal(err)
	}
	if l := c.Lights[0]; l.Name != "Sofa" || l.Icon != "mdi:sofa" || len(c.Windows) != 1 {
		t.Errorf("got %+v %+v", c.Lights, c.Windows)
	}
}

func TestStandbyLabels(t *testing.T) {
	c, err := Parse(strings.NewReader("[standby]" + string(rune(10)) + "indoor = sensor.a | Studio" + string(rune(10)) + "outdoor = sensor.b"))
	if err != nil {
		t.Fatal(err)
	}
	if s := c.Screensaver; s.Indoor != (Item{Entity: "sensor.a", Name: "Studio"}) || s.Outdoor != (Item{Entity: "sensor.b", Name: "Esterno"}) {
		t.Errorf("%+v", s)
	}
}
