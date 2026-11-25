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
