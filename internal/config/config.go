// Package config reads the dashboard's INI-style text file.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// Item is one entity shown on screen. Name may be empty: the Home Assistant
// friendly name is used then.
type Item struct {
	Entity string
	Name   string
}

type Config struct {
	URL   string
	Token string

	Orientation      string // portrait, portrait_inverted, landscape, landscape_inverted
	ScreenRotation   int    // -1 = auto, otherwise 0/90/180/270 applied to the framebuffer
	FullRefreshEvery time.Duration
	Timezone         string

	TouchDevice  string
	TouchSwapXY  bool
	TouchMirrorX bool
	TouchMirrorY bool

	Sensors []Item
	Lights  []Item
	Climate []Item
}

// Default returns the settings that match a Kobo Glo (N613).
func Default() Config {
	return Config{
		URL:              "http://homeassistant.local:8123",
		Orientation:      "landscape",
		ScreenRotation:   -1,
		FullRefreshEvery: 30 * time.Minute,
		Timezone:         "Europe/Rome",
		TouchDevice:      "/dev/input/event1",
		TouchSwapXY:      true,
		TouchMirrorX:     true,
	}
}

// Entities lists every configured entity once.
func (c Config) Entities() []string {
	seen := map[string]bool{}
	var out []string
	for _, list := range [][]Item{c.Sensors, c.Lights, c.Climate} {
		for _, it := range list {
			if !seen[it.Entity] {
				seen[it.Entity] = true
				out = append(out, it.Entity)
			}
		}
	}
	return out
}

func Load(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	return Parse(f)
}

func Parse(r io.Reader) (Config, error) {
	c := Default()
	section := ""
	sc := bufio.NewScanner(r)
	for n := 1; sc.Scan(); n++ {
		// Windows editors may add a BOM at the start of the file.
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\uFEFF"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			switch section {
			case "homeassistant", "display", "touch", "sensors", "lights", "climate":
			default:
				return c, fmt.Errorf("line %d: unknown section [%s]", n, section)
			}
			continue
		}
		key, val, hasVal := strings.Cut(line, "=")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)

		switch section {
		case "sensors", "lights", "climate":
			if !strings.Contains(key, ".") {
				return c, fmt.Errorf("line %d: %q is not an entity id", n, key)
			}
			it := Item{Entity: key, Name: val}
			switch section {
			case "sensors":
				c.Sensors = append(c.Sensors, it)
			case "lights":
				c.Lights = append(c.Lights, it)
			default:
				c.Climate = append(c.Climate, it)
			}
			continue
		}

		if !hasVal {
			return c, fmt.Errorf("line %d: expected key = value", n)
		}
		if err := c.set(section, strings.ToLower(key), val); err != nil {
			return c, fmt.Errorf("line %d: %w", n, err)
		}
	}
	return c, sc.Err()
}

func (c *Config) set(section, key, val string) error {
	var err error
	switch section + "." + key {
	case "homeassistant.url":
		c.URL = strings.TrimRight(val, "/")
	case "homeassistant.token":
		c.Token = val
	case "display.orientation":
		switch val {
		case "portrait", "portrait_inverted", "landscape", "landscape_inverted":
			c.Orientation = val
		default:
			return fmt.Errorf("invalid orientation %q", val)
		}
	case "display.screen_rotation":
		switch val {
		case "auto":
			c.ScreenRotation = -1
		case "0", "90", "180", "270":
			c.ScreenRotation, _ = strconv.Atoi(val)
		default:
			return fmt.Errorf("screen_rotation must be auto, 0, 90, 180 or 270")
		}
	case "display.full_refresh_every":
		c.FullRefreshEvery, err = time.ParseDuration(val)
	case "display.timezone":
		c.Timezone = val
	case "touch.device":
		c.TouchDevice = val
	case "touch.swap_xy":
		c.TouchSwapXY, err = strconv.ParseBool(val)
	case "touch.mirror_x":
		c.TouchMirrorX, err = strconv.ParseBool(val)
	case "touch.mirror_y":
		c.TouchMirrorY, err = strconv.ParseBool(val)
	default:
		return fmt.Errorf("unknown setting %q in [%s]", key, section)
	}
	return err
}
