package ha

import (
	"encoding/json"
	"slices"
	"time"
)

// Event is one calendar entry. For all-day events only the dates of Start and
// End are meaningful, and End is the day after the last one.
type Event struct {
	Calendar   string // entity id
	Summary    string
	Start, End time.Time
	AllDay     bool
}

// calendarRequest asks for the events from the start of the current month, so
// that the whole month grid can be marked, to 45 days ahead.
func calendarRequest(calendars []string, now time.Time) map[string]any {
	from := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return map[string]any{
		"type":    "call_service",
		"domain":  "calendar",
		"service": "get_events",
		"target":  map[string]any{"entity_id": calendars},
		"service_data": map[string]any{
			"start_date_time": from.Format(time.RFC3339),
			"end_date_time":   now.AddDate(0, 0, 45).Format(time.RFC3339),
		},
		"return_response": true,
	}
}

func parseEventTime(s string) (t time.Time, allDay, ok bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, true
	}
	t, err := time.Parse(time.DateOnly, s)
	return t, true, err == nil
}

// parseEvents reads the service response, sorted by start.
func parseEvents(result []byte) ([]Event, error) {
	var raw struct {
		Response map[string]struct {
			Events []struct {
				Start   string `json:"start"`
				End     string `json:"end"`
				Summary string `json:"summary"`
			} `json:"events"`
		} `json:"response"`
	}
	if err := json.Unmarshal(result, &raw); err != nil {
		return nil, err
	}
	var out []Event
	for calendar, list := range raw.Response {
		for _, e := range list.Events {
			start, allDay, ok1 := parseEventTime(e.Start)
			end, _, ok2 := parseEventTime(e.End)
			if ok1 && ok2 {
				out = append(out, Event{calendar, e.Summary, start, end, allDay})
			}
		}
	}
	slices.SortFunc(out, func(a, b Event) int {
		if c := a.Start.Compare(b.Start); c != 0 {
			return c
		}
		if a.Calendar < b.Calendar {
			return -1
		}
		if a.Calendar > b.Calendar {
			return 1
		}
		return 0
	})
	return out, nil
}

// Events returns the last events fetched. The slice is never modified.
func (c *Client) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.events
}
