package ha

import (
	"encoding/json"
	"testing"
)

func TestApplyCompressedEvents(t *testing.T) {
	states := map[string]State{}
	feed := func(s string) {
		var ev entitiesEvent
		if err := json.Unmarshal([]byte(s), &ev); err != nil {
			t.Fatal(err)
		}
		apply(states, &ev)
	}

	feed(`{"a":{"climate.x":{"s":"off","a":{"temperature":23,"preset_mode":"none"}}}}`)
	before := states["climate.x"]
	feed(`{"c":{"climate.x":{"+":{"s":"cool","a":{"temperature":22}},"-":{"a":["preset_mode"]}}}}`)

	got := states["climate.x"]
	if got.State != "cool" {
		t.Errorf("state = %q", got.State)
	}
	if v, _ := got.Float("temperature"); v != 22 {
		t.Errorf("temperature = %v", v)
	}
	if got.String("preset_mode") != "" {
		t.Error("preset_mode should have been removed")
	}
	if v, _ := before.Float("temperature"); v != 23 {
		t.Error("earlier snapshot was modified")
	}

	feed(`{"r":["climate.x"]}`)
	if len(states) != 0 {
		t.Error("entity not removed")
	}
}

func TestWebsocketURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://192.168.1.10:8123/": "ws://192.168.1.10:8123/api/websocket",
		"https://ha.example":        "wss://ha.example/api/websocket",
		"ha.local:8123":             "ws://ha.local:8123/api/websocket",
	} {
		if got := websocketURL(in); got != want {
			t.Errorf("websocketURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseHistory(t *testing.T) {
	got, err := parseHistory([]byte(`{"sensor.x":[{"s":"19.5","a":{},"lu":1000.5},{"s":"unavailable","lu":1001},{"s":"20","lu":1002}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if p := got["sensor.x"]; len(p) != 2 || p[0].Value != 19.5 || p[1].Time.Unix() != 1002 {
		t.Errorf("got %+v", p)
	}
}

func TestParseEvents(t *testing.T) {
	got, err := parseEvents([]byte(`{"context":{},"response":{"calendar.a":{"events":[
		{"start":"2026-11-27","end":"2026-11-28","summary":"all day"},
		{"start":"2026-11-26T10:00:00+01:00","end":"2026-11-26T11:00:00+01:00","summary":"timed"}]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Summary != "timed" || got[0].AllDay || !got[1].AllDay {
		t.Errorf("got %+v", got)
	}
}
