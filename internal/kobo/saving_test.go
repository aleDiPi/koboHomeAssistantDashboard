package kobo

import "testing"

func TestGovernor(t *testing.T) {
	all := []string{"userspace", "ondemand", "conservative", "performance"}
	if g := governor(all, true, "userspace"); g != "conservative" {
		t.Errorf("saving: %s", g)
	}
	if g := governor(all, false, "userspace"); g != "userspace" {
		t.Errorf("restore: %s", g)
	}
	if g := governor([]string{"userspace"}, true, "userspace"); g != "userspace" {
		t.Errorf("nothing frugal: %s", g)
	}
}

func TestVolts(t *testing.T) {
	for raw, want := range map[string]float64{"4012000" + string(rune(10)): 4.012, "3950": 3.95, "x": 0} {
		if got := volts(raw); got != want {
			t.Errorf("volts(%q) = %v", raw, got)
		}
	}
}
