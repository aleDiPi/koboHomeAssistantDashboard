package ha

import "testing"

func TestParseDashboard(t *testing.T) {
	l, err := parseDashboard([]byte(`{"views":[
	 {"title":"Casa","type":"sections",
	  "badges":["binary_sensor.a",{"type":"entity","entity":"cover.b","name":"Roof"}],
	  "sections":[{"cards":[
	    {"type":"heading","heading":"Temps"},
	    {"type":"sensor","entity":"sensor.t","name":"Out"},
	    {"type":"calendar","entities":["calendar.c"]}]}]},
	 {"title":"Mixed","cards":[
	  {"type":"vertical-stack","cards":[
	    {"type":"tile","entity":"light.x","icon":"mdi:sofa","name":{"type":"entity"}},
	    {"type":"entities","entities":["switch.y",{"entity":"climate.z","name":"AC"},"weather.skipped"]}]},
	  {"type":"conditional","card":{"type":"tile","entity":"cover.v"}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Pages) != 2 || len(l.Notifications) != 2 || l.Notifications[1].Name != "Roof" {
		t.Fatalf("%+v", l)
	}
	home, mixed := l.Pages[0], l.Pages[1]
	if home.Sensors[0].Name != "Out" || home.Calendars[0].Entity != "calendar.c" {
		t.Errorf("home %+v", home)
	}
	if len(mixed.Lights) != 2 || mixed.Lights[0].Icon != "mdi:sofa" || mixed.Climate[0].Name != "AC" || len(mixed.Windows) != 1 {
		t.Errorf("mixed %+v", mixed)
	}
	if _, err := parseDashboard([]byte(`{"views":[]}`)); err == nil {
		t.Error("an empty dashboard must be rejected")
	}
}

func TestScriptWithTimer(t *testing.T) {
	l, err := parseDashboard([]byte(`{"views":[{"cards":[
	 {"type":"tile","entity":"script.air","kobo_timer":"input_number.minutes"},
	 {"type":"tile","entity":"script.other","kobo_timer":5}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	a := l.Pages[0].Actions
	if len(a) != 2 || a[0].Timer != "input_number.minutes" || a[1].Timer != "5" || len(l.Pages[0].Lights) != 0 {
		t.Errorf("%+v", l.Pages[0])
	}
}
