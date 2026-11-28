package ha

// Reads the pages of the dashboard from a Home Assistant (Lovelace) dashboard,
// so that they can be edited in the Home Assistant interface. Each view is a
// page; of each card only the entities, names and icons matter, and the entity
// domain decides how it is drawn. The badges become the notification chips.

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/aleDiPi/koboHomeAssistantDashboard/internal/config"
)

// maxPages is how many buttons fit in the bottom bar.
const maxPages = 6

type llCard struct {
	Entity string `json:"entity"`
	Name   any    `json:"name"`
	Icon   string `json:"icon"`
	// Our own key: the countdown of a script, in minutes or as an entity.
	Timer    any               `json:"kobo_timer"`
	Entities []json.RawMessage `json:"entities"`
	Cards    []llCard          `json:"cards"`
	Card     *llCard           `json:"card"` // conditional cards
}

type llView struct {
	Title    string            `json:"title"`
	Badges   []json.RawMessage `json:"badges"`
	Cards    []llCard          `json:"cards"`
	Sections []struct {
		Cards []llCard `json:"cards"`
	} `json:"sections"`
}

func timer(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return ""
}

// reference reads "light.x" or {"entity": "light.x", "name": ..., "icon": ...}.
func reference(raw json.RawMessage) (config.Item, bool) {
	var id string
	if json.Unmarshal(raw, &id) == nil {
		return config.Item{Entity: id}, strings.Contains(id, ".")
	}
	var c llCard
	if json.Unmarshal(raw, &c) != nil {
		return config.Item{}, false
	}
	name, _ := c.Name.(string)
	return config.Item{Entity: c.Entity, Name: name, Icon: c.Icon, Timer: timer(c.Timer)}, strings.Contains(c.Entity, ".")
}

func (c llCard) items(out []config.Item) []config.Item {
	name, _ := c.Name.(string)
	if strings.Contains(c.Entity, ".") {
		out = append(out, config.Item{Entity: c.Entity, Name: name, Icon: c.Icon, Timer: timer(c.Timer)})
	}
	for _, raw := range c.Entities {
		if it, ok := reference(raw); ok {
			out = append(out, it)
		}
	}
	for _, child := range c.Cards {
		out = child.items(out)
	}
	if c.Card != nil {
		out = c.Card.items(out)
	}
	return out
}

func addToPage(p *config.Page, it config.Item) {
	domain, _, _ := strings.Cut(it.Entity, ".")
	var list *[]config.Item
	switch domain {
	case "sensor", "input_number", "number":
		list = &p.Sensors
	case "light", "switch", "input_boolean", "fan":
		list = &p.Lights
	case "script", "scene", "button", "input_button":
		list = &p.Actions
	case "climate":
		list = &p.Climate
	case "cover", "binary_sensor":
		list = &p.Windows
	case "calendar":
		list = &p.Calendars
	default:
		return
	}
	for _, have := range *list {
		if have.Entity == it.Entity {
			return
		}
	}
	*list = append(*list, it)
}

// parseDashboard converts the answer to "lovelace/config".
func parseDashboard(result []byte) (config.Layout, error) {
	var raw struct {
		Views []llView `json:"views"`
	}
	var layout config.Layout
	if err := json.Unmarshal(result, &raw); err != nil {
		return layout, err
	}
	notified := map[string]bool{}
	items := 0
	for n, v := range raw.Views {
		for _, b := range v.Badges {
			if it, ok := reference(b); ok && !notified[it.Entity] {
				notified[it.Entity] = true
				layout.Notifications = append(layout.Notifications, it)
			}
		}
		if len(layout.Pages) == maxPages {
			continue
		}
		page := config.Page{Title: v.Title}
		if page.Title == "" {
			page.Title = "Pagina " + strconv.Itoa(n+1)
		}
		var found []config.Item
		for _, c := range v.Cards {
			found = c.items(found)
		}
		for _, s := range v.Sections {
			for _, c := range s.Cards {
				found = c.items(found)
			}
		}
		for _, it := range found {
			addToPage(&page, it)
		}
		items += len(found)
		layout.Pages = append(layout.Pages, page)
	}
	if items == 0 {
		return layout, errors.New("the dashboard has no entities")
	}
	return layout, nil
}
