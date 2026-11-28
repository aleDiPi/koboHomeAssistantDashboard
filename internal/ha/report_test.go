package ha

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
)

func TestReport(t *testing.T) {
	var path, auth, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		path, auth, body = r.URL.Path, r.Header.Get("Authorization"), string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.URL, cfg.Token = srv.URL, "secret"
	err := NewClient(cfg).Report([]Sensor{{Entity: "sensor.x", State: "80", Attr: map[string]any{"unit_of_measurement": "%"}}})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/states/sensor.x" || auth != "Bearer secret" || body != `{"state":"80","attributes":{"unit_of_measurement":"%"}}` {
		t.Errorf("%s %s %s", path, auth, body)
	}
}
