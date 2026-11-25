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
