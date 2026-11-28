package ha

// Publishes the device's own sensors through the REST API. Such states need no
// setup in Home Assistant; they vanish when it restarts, until the next report.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Sensor is one value published to Home Assistant.
type Sensor struct {
	Entity string         `json:"-"`
	State  string         `json:"state"`
	Attr   map[string]any `json:"attributes"`
}

var reportClient = &http.Client{Timeout: 10 * time.Second}

// Report publishes the sensors one by one.
func (c *Client) Report(sensors []Sensor) error {
	base := strings.TrimRight(c.cfg.URL, "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	for _, s := range sensors {
		body, err := json.Marshal(s)
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, base+"/api/states/"+s.Entity, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := reportClient.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return fmt.Errorf("%s: %s", s.Entity, resp.Status)
		}
	}
	return nil
}
