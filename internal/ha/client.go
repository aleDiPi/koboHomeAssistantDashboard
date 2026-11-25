// Package ha talks to Home Assistant over its WebSocket API.
package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
)

// State is the current value of one entity.
type State struct {
	State string
	Attr  map[string]any
}

func (s State) Float(key string) (float64, bool) {
	v, ok := s.Attr[key].(float64)
	return v, ok
}

func (s State) String(key string) string {
	v, _ := s.Attr[key].(string)
	return v
}

func (s State) Strings(key string) []string {
	list, _ := s.Attr[key].([]any)
	var out []string
	for _, v := range list {
		if str, ok := v.(string); ok {
			out = append(out, str)
		}
	}
	return out
}

const (
	pingEvery  = 30 * time.Second
	idleLimit  = 75 * time.Second
	maxBackoff = 30 * time.Second
	retryFetch = 30 * time.Second
)

// errDashboardChanged restarts the session to load the new pages.
var errDashboardChanged = errors.New("dashboard changed")

// ErrAuth means Home Assistant rejected the token.
var ErrAuth = errors.New("token rejected")

// Client keeps a subscription to a fixed set of entities and reconnects on
// its own.
type Client struct {
	url, token string
	cfg        config.Config
	layout     config.Layout
	history    map[string][]Point
	events     []Event

	mu        sync.Mutex
	conn      *wsConn // nil while disconnected
	nextID    int
	states    map[string]State
	connected bool
	lastErr   error

	kick chan struct{} // cuts the wait before reconnecting

	// Changed receives a value (without blocking) whenever states or the
	// connection status change.
	Changed chan struct{}
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		url:     websocketURL(cfg.URL),
		token:   cfg.Token,
		cfg:     cfg,
		layout:  cfg.LocalLayout(),
		states:  map[string]State{},
		Changed: make(chan struct{}, 1),
		kick:    make(chan struct{}, 1),
	}
}

// Layout returns the pages in use. It is never modified in place.
func (c *Client) Layout() config.Layout {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.layout
}

func websocketURL(base string) string {
	base = strings.TrimRight(base, "/")
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + base[len("https://"):]
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + base[len("http://"):]
	default:
		base = "ws://" + base
	}
	return base + "/api/websocket"
}

// Snapshot returns a copy of the known states.
func (c *Client) Snapshot() (states map[string]State, connected bool, lastErr error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	states = make(map[string]State, len(c.states))
	for k, v := range c.states {
		states[k] = v
	}
	return states, c.connected, c.lastErr
}

func (c *Client) notify() {
	select {
	case c.Changed <- struct{}{}:
	default:
	}
}

// Run connects and reconnects until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := c.session(ctx)
		if errors.Is(err, errDashboardChanged) {
			log.Print("home assistant: dashboard changed, reloading")
			continue
		}
		c.mu.Lock()
		c.conn, c.connected, c.lastErr = nil, false, err
		c.mu.Unlock()
		c.notify()
		if ctx.Err() != nil {
			return
		}
		log.Printf("home assistant: %v", err)
		switch {
		case errors.Is(err, ErrAuth):
			// Retrying fast with a bad token would get the Kobo's IP banned.
			backoff = 5 * time.Minute
		case time.Since(start) > time.Minute:
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		case <-c.kick:
			backoff = time.Second
			continue
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

type message struct {
	ID      int             `json:"id"`
	Type    string          `json:"type"`
	Success *bool           `json:"success"`
	Error   json.RawMessage `json:"error"`
	Event   json.RawMessage `json:"event"`
	Result  json.RawMessage `json:"result"`
}

// entitiesEvent is the compressed format of subscribe_entities.
type entitiesEvent struct {
	Added   map[string]compactState `json:"a"`
	Changed map[string]struct {
		Plus  *compactState `json:"+"`
		Minus *struct {
			Attr []string `json:"a"`
		} `json:"-,"`
	} `json:"c"`
	Removed []string `json:"r"`
}

type compactState struct {
	State *string        `json:"s"`
	Attr  map[string]any `json:"a"`
}

// apply merges an event into states. Attribute maps are replaced, never
// modified, so that snapshots stay safe to read.
func apply(states map[string]State, ev *entitiesEvent) {
	for id, s := range ev.Added {
		st := State{Attr: s.Attr}
		if s.State != nil {
			st.State = *s.State
		}
		states[id] = st
	}
	for id, diff := range ev.Changed {
		old := states[id]
		attr := make(map[string]any, len(old.Attr))
		for k, v := range old.Attr {
			attr[k] = v
		}
		if p := diff.Plus; p != nil {
			if p.State != nil {
				old.State = *p.State
			}
			for k, v := range p.Attr {
				attr[k] = v
			}
		}
		if m := diff.Minus; m != nil {
			for _, k := range m.Attr {
				delete(attr, k)
			}
		}
		old.Attr = attr
		states[id] = old
	}
	for _, id := range ev.Removed {
		delete(states, id)
	}
}

func readJSON(w *wsConn, v any) error {
	p, err := w.ReadMessage(idleLimit)
	if err != nil {
		return err
	}
	return json.Unmarshal(p, v)
}

func writeJSON(w *wsConn, v any) error {
	p, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteText(p)
}

func (c *Client) session(ctx context.Context) error {
	w, err := dialWS(c.url)
	if err != nil {
		return err
	}
	defer w.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			w.Close()
		case <-done:
		}
	}()

	var m message
	if err := readJSON(w, &m); err != nil {
		return err
	}
	if m.Type != "auth_required" {
		return fmt.Errorf("unexpected greeting %q", m.Type)
	}
	if err := writeJSON(w, map[string]any{"type": "auth", "access_token": c.token}); err != nil {
		return err
	}
	if err := readJSON(w, &m); err != nil {
		return err
	}
	if m.Type != "auth_ok" {
		return ErrAuth
	}

	c.mu.Lock()
	c.conn, c.nextID = w, 0
	c.connected, c.lastErr = true, nil
	c.states = map[string]State{}
	c.mu.Unlock()

	updatesID := 0
	if c.cfg.Dashboard != "" {
		layout, err := c.fetchDashboard(w)
		if err != nil {
			return err
		}
		c.mu.Lock()
		c.layout = layout
		c.mu.Unlock()
		updatesID, err = c.send(map[string]any{"type": "subscribe_events", "event_type": "lovelace_updated"})
		if err != nil {
			return err
		}
	}
	layout := c.Layout()
	graphed, calendars := layout.Graphed(), layout.CalendarIDs()
	subID, err := c.send(map[string]any{"type": "subscribe_entities", "entity_ids": c.cfg.Watched(layout)})
	if err != nil {
		return err
	}
	c.notify()

	var historyID atomic.Int64
	fetchHistory := func() error {
		if len(graphed) == 0 {
			return nil
		}
		id, err := c.send(historyRequest(graphed, time.Now()))
		historyID.Store(int64(id))
		return err
	}
	var eventsID atomic.Int64
	fetchEvents := func() error {
		if len(calendars) == 0 {
			return nil
		}
		id, err := c.send(calendarRequest(calendars, time.Now()))
		eventsID.Store(int64(id))
		return err
	}
	if err := fetchHistory(); err != nil {
		return err
	}
	if err := fetchEvents(); err != nil {
		return err
	}

	go func() {
		t := time.NewTicker(pingEvery)
		defer t.Stop()
		h := time.NewTicker(historyEvery)
		defer h.Stop()
		for {
			select {
			case <-done:
				return
			case <-h.C:
				if err := errors.Join(fetchHistory(), fetchEvents()); err != nil {
					w.Close()
					return
				}
			case <-t.C:
				if _, err := c.send(map[string]any{"type": "ping"}); err != nil {
					w.Close()
					return
				}
			}
		}
	}()

	for {
		var m message
		if err := readJSON(w, &m); err != nil {
			return err
		}
		switch {
		case m.Type == "event" && m.ID == subID:
			var ev entitiesEvent
			if err := json.Unmarshal(m.Event, &ev); err != nil {
				return err
			}
			c.mu.Lock()
			apply(c.states, &ev)
			c.mu.Unlock()
			c.notify()
		case m.Type == "event" && updatesID != 0 && m.ID == updatesID:
			var ev struct {
				Data struct {
					URLPath string `json:"url_path"`
				} `json:"data"`
			}
			if json.Unmarshal(m.Event, &ev) == nil && ev.Data.URLPath == c.cfg.Dashboard {
				return errDashboardChanged
			}
		case m.Type == "result" && int64(m.ID) == historyID.Load() && m.Success != nil && *m.Success:
			history, err := parseHistory(m.Result)
			if err != nil {
				log.Printf("home assistant: history: %v", err)
				continue
			}
			c.mu.Lock()
			c.history = history
			c.mu.Unlock()
			c.notify()
		case m.Type == "result" && int64(m.ID) == eventsID.Load() && m.Success != nil && *m.Success:
			events, err := parseEvents(m.Result)
			if err != nil {
				log.Printf("home assistant: calendar: %v", err)
				continue
			}
			c.mu.Lock()
			c.events = events
			c.mu.Unlock()
			c.notify()
		case m.Type == "result" && m.Success != nil && !*m.Success && (int64(m.ID) == historyID.Load() || int64(m.ID) == eventsID.Load()):
			// While Home Assistant is still starting, history and calendars
			// are not there yet: ask again soon.
			log.Printf("home assistant: command %d failed, retrying: %s", m.ID, m.Error)
			again := fetchHistory
			if int64(m.ID) == eventsID.Load() {
				again = fetchEvents
			}
			time.AfterFunc(retryFetch, func() {
				select {
				case <-done:
				default:
					if err := again(); err != nil {
						w.Close()
					}
				}
			})
		case m.Type == "result" && m.Success != nil && !*m.Success:
			if m.ID == subID {
				return fmt.Errorf("subscription refused: %s", m.Error)
			}
			log.Printf("home assistant: command %d failed: %s", m.ID, m.Error)
		}
	}
}

// fetchDashboard reads the pages from Home Assistant. It runs before any
// subscription, so the next result on the wire answers it. A dashboard that is
// missing or unusable leaves the lists of the configuration file in charge.
func (c *Client) fetchDashboard(w *wsConn) (config.Layout, error) {
	local := c.cfg.LocalLayout()
	id, err := c.send(map[string]any{"type": "lovelace/config", "url_path": c.cfg.Dashboard, "force": false})
	if err != nil {
		return local, err
	}
	for {
		var m message
		if err := readJSON(w, &m); err != nil {
			return local, err
		}
		if m.ID != id || m.Type != "result" {
			continue
		}
		if m.Success == nil || !*m.Success {
			// Right after a restart the command does not exist yet: try the
			// whole session again rather than showing the local pages.
			if strings.Contains(string(m.Error), "unknown_command") {
				return local, errors.New("Home Assistant is still starting")
			}
			log.Printf("home assistant: dashboard %q: %s", c.cfg.Dashboard, m.Error)
			return local, nil
		}
		layout, err := parseDashboard(m.Result)
		if err != nil {
			log.Printf("home assistant: dashboard %q: %v", c.cfg.Dashboard, err)
			return local, nil
		}
		return layout, nil
	}
}

// send assigns the message id and writes in one critical section, because
// Home Assistant requires ids to increase on the wire.
func (c *Client) send(msg map[string]any) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return 0, errors.New("not connected")
	}
	c.nextID++
	msg["id"] = c.nextID
	return c.nextID, writeJSON(c.conn, msg)
}

// Call runs a service on one entity, e.g. Call("light", "toggle", id, nil).
func (c *Client) Call(domain, service, entity string, data map[string]any) error {
	msg := map[string]any{
		"type":    "call_service",
		"domain":  domain,
		"service": service,
		"target":  map[string]any{"entity_id": entity},
	}
	if len(data) > 0 {
		msg["service_data"] = data
	}
	_, err := c.send(msg)
	return err
}

// Reconnect drops the connection and dials again at once: after a suspend the
// old one is dead but would take a minute to be noticed.
func (c *Client) Reconnect() {
	c.mu.Lock()
	conn := c.conn
	c.connected = false
	c.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

// Updates is signalled whenever Snapshot would return something new.
func (c *Client) Updates() <-chan struct{} { return c.Changed }
