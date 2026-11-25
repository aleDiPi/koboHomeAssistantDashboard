package ha

import (
	"encoding/json"
	"strconv"
	"time"
)

// Point is one past value of a numeric sensor.
type Point struct {
	Time  time.Time
	Value float64
}

const (
	// HistorySpan is the period covered by the graphs.
	HistorySpan  = 24 * time.Hour
	historyEvery = 10 * time.Minute
)

// historyRequest asks for the recorded values of the graphed sensors.
func historyRequest(entities []string, now time.Time) map[string]any {
	return map[string]any{
		"type":                     "history/history_during_period",
		"start_time":               now.Add(-HistorySpan).UTC().Format(time.RFC3339),
		"entity_ids":               entities,
		"minimal_response":         true,
		"no_attributes":            true,
		"significant_changes_only": false,
	}
}

// parseHistory reads the compressed result, skipping non-numeric states such
// as "unavailable".
func parseHistory(result []byte) (map[string][]Point, error) {
	var raw map[string][]struct {
		State   string  `json:"s"`
		Updated float64 `json:"lu"`
	}
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, err
	}
	out := make(map[string][]Point, len(raw))
	for id, list := range raw {
		for _, e := range list {
			v, err := strconv.ParseFloat(e.State, 64)
			if err != nil {
				continue
			}
			out[id] = append(out[id], Point{time.Unix(int64(e.Updated), 0), v})
		}
	}
	return out, nil
}

// History returns the last values fetched. The slices are never modified.
func (c *Client) History() map[string][]Point {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.history
}
