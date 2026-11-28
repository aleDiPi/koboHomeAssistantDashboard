// Package config reads the dashboard's INI-style text file.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Item is one entity shown on screen. Name may be empty: the Home Assistant
// friendly name is used then.
type Item struct {
	Entity string
	Name   string
	Icon   string // optional Material Design icon, as in "mdi:floor-lamp"
	// Timer gives a script a countdown: minutes, or the entity holding them.
	Timer string
}

type Config struct {
	URL   string
	Token string
	// Dashboard is the url path of the Home Assistant dashboard that defines
	// the pages; empty to use the lists below.
	Dashboard string

	Orientation      string // portrait, portrait_inverted, landscape, landscape_inverted
	ScreenRotation   int    // -1 = auto, otherwise 0/90/180/270 applied to the framebuffer
	FullRefreshEvery time.Duration
	Timezone         string
	ReturnHome       time.Duration // back to the home view when idle; 0 = never

	TouchDevice  string
	TouchSwapXY  bool
	TouchMirrorX bool
	TouchMirrorY bool

	Frontlight  Frontlight
	Power       Power
	Update      Update
	Screensaver Screensaver

	// Shown as chips in the header while on (or open).
	Notifications []Item

	Actions   []Item // scripts, scenes and buttons
	Calendars []Item
	Windows   []Item // contact sensors and motorised covers
	Sensors   []Item
	Lights    []Item
	Climate   []Item
}

// set reads "entity" or "entity | label".
func (it *Item) set(val string) {
	entity, label, _ := strings.Cut(val, "|")
	it.Entity = strings.TrimSpace(entity)
	if label = strings.TrimSpace(label); label != "" {
		it.Name = label
	}
}

// Frontlight controls the screen light.
type Frontlight struct {
	Manage     bool // false leaves the light as the Kobo had it
	NightLevel int  // after sunset, with every NightWhenOff light off
	TouchLevel int  // after sunset, for TouchTime after a touch
	TouchTime  time.Duration
	// Lights that make the screen light unnecessary when any of them is on.
	NightWhenOff []string
	Sun          string
}

// Power controls standby.
type Power struct {
	Standby bool
	// Saving rests processor and Wi-Fi while on battery.
	Saving bool
	// People (or trackers) whose being "home" means somebody can look at it.
	Presence    []string
	Idle        time.Duration // without touches before sleeping
	HomeRefresh time.Duration // at home, on battery
	AwayRefresh time.Duration // with nobody at home
	// Report publishes battery, power and light to Home Assistant; on battery
	// only every ReportOnBattery (0 = never).
	Report          bool
	ReportOnBattery time.Duration
}

// Screensaver is what the standby screen shows.
type Screensaver struct {
	// Picture is a PNG or JPEG file, or a folder of them taking turns day by
	// day; relative paths start from the folder of the configuration file.
	Picture string
	// Focus chooses what survives the crop: 0 left or top, 1 right or bottom.
	Focus   float64
	Indoor  Item // temperature sensor inside, with its label
	Outdoor Item
}

// Update says where new versions come from. Without a key it is off.
type Update struct {
	URL        string // folder holding version.txt and the signed archive
	Key        string // Ed25519 public key, base64
	CheckEvery time.Duration
}

// Default returns the settings that match a Kobo Glo (N613).
func Default() Config {
	return Config{
		URL:              "http://homeassistant.local:8123",
		Orientation:      "landscape",
		ScreenRotation:   -1,
		FullRefreshEvery: 30 * time.Minute,
		Timezone:         "Europe/Rome",
		ReturnHome:       2 * time.Minute,
		TouchDevice:      "/dev/input/event1",
		TouchSwapXY:      true,
		TouchMirrorX:     true,
		Power:            Power{Idle: 2 * time.Minute, HomeRefresh: 5 * time.Minute, AwayRefresh: 15 * time.Minute, ReportOnBattery: 15 * time.Minute},
		Screensaver:      Screensaver{Focus: 0.5, Indoor: Item{Name: "Interno"}, Outdoor: Item{Name: "Esterno"}},
		Update: Update{
			URL:        "https://github.com/aleDiPi/koboHomeAssistantDashboard/releases/latest/download",
			CheckEvery: time.Hour,
		},
		Frontlight: Frontlight{
			Manage: true, NightLevel: 1, TouchLevel: 15,
			TouchTime: 30 * time.Second, Sun: "sun.sun",
		},
	}
}

// SecretsFile, next to the configuration, holds what must not be shared.
const SecretsFile = "secrets.ini"

// Load reads the configuration and then, if present, the secrets file, whose
// non-empty values win.
func Load(path string) (Config, error) {
	c := Default()
	if err := parseFile(&c, path, false); err != nil {
		return c, err
	}
	secrets := filepath.Join(filepath.Dir(path), SecretsFile)
	if err := parseFile(&c, secrets, true); err != nil && !errors.Is(err, os.ErrNotExist) {
		return c, fmt.Errorf("%s: %w", SecretsFile, err)
	}
	return c, nil
}

func parseFile(c *Config, path string, skipEmpty bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return parseInto(c, f, skipEmpty)
}

func Parse(r io.Reader) (Config, error) {
	c := Default()
	err := parseInto(&c, r, false)
	return c, err
}

// parseInto applies the file to c. With skipEmpty, a setting left blank keeps
// the value it had.
func parseInto(c *Config, r io.Reader, skipEmpty bool) error {
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
			case "homeassistant", "display", "touch", "frontlight", "update", "power", "standby", "notifications", "calendar", "windows", "actions", "sensors", "lights", "climate":
			default:
				return fmt.Errorf("line %d: unknown section [%s]", n, section)
			}
			continue
		}
		key, val, hasVal := strings.Cut(line, "=")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)

		switch section {
		case "sensors", "lights", "climate", "notifications", "calendar", "windows", "actions":
			if !strings.Contains(key, ".") {
				return fmt.Errorf("line %d: %q is not an entity id", n, key)
			}
			// name | mdi:icon | timer, the last two optional and in any order
			fields := strings.Split(val, "|")
			it := Item{Entity: key, Name: strings.TrimSpace(fields[0])}
			for _, field := range fields[1:] {
				if field = strings.TrimSpace(field); strings.HasPrefix(field, "mdi:") {
					it.Icon = field
				} else {
					it.Timer = field
				}
			}
			switch section {
			case "sensors":
				c.Sensors = append(c.Sensors, it)
			case "actions":
				c.Actions = append(c.Actions, it)
			case "windows":
				c.Windows = append(c.Windows, it)
			case "calendar":
				c.Calendars = append(c.Calendars, it)
			case "notifications":
				c.Notifications = append(c.Notifications, it)
			case "lights":
				c.Lights = append(c.Lights, it)
			default:
				c.Climate = append(c.Climate, it)
			}
			continue
		}

		if skipEmpty && val == "" {
			continue
		}
		if !hasVal {
			return fmt.Errorf("line %d: expected key = value", n)
		}
		if err := c.set(section, strings.ToLower(key), val); err != nil {
			return fmt.Errorf("line %d: %w", n, err)
		}
	}
	return sc.Err()
}

func (c *Config) set(section, key, val string) error {
	var err error
	switch section + "." + key {
	case "homeassistant.url":
		c.URL = strings.TrimRight(val, "/")
	case "homeassistant.dashboard":
		c.Dashboard = val
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
	case "display.return_home":
		c.ReturnHome, err = time.ParseDuration(val)
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
	case "power.saving":
		c.Power.Saving, err = strconv.ParseBool(val)
	case "power.report":
		c.Power.Report, err = strconv.ParseBool(val)
	case "power.report_on_battery":
		c.Power.ReportOnBattery, err = time.ParseDuration(val)
	case "standby.picture":
		c.Screensaver.Picture = val
	case "standby.focus":
		c.Screensaver.Focus, err = strconv.ParseFloat(val, 64)
		if err == nil && !(c.Screensaver.Focus >= 0 && c.Screensaver.Focus <= 1) {
			err = fmt.Errorf("focus must be between 0 and 1")
		}
	case "standby.indoor":
		c.Screensaver.Indoor.set(val)
	case "standby.outdoor":
		c.Screensaver.Outdoor.set(val)
	case "update.url":
		c.Update.URL = val
	case "update.key":
		c.Update.Key = val
	case "update.check_every":
		c.Update.CheckEvery, err = atLeast(val, 5*time.Minute)
	case "power.standby":
		c.Power.Standby, err = strconv.ParseBool(val)
	case "power.presence":
		c.Power.Presence = strings.Fields(strings.ReplaceAll(val, ",", " "))
	case "power.idle":
		c.Power.Idle, err = atLeast(val, 10*time.Second)
	case "power.home_refresh":
		c.Power.HomeRefresh, err = atLeast(val, time.Minute)
	case "power.away_refresh":
		c.Power.AwayRefresh, err = atLeast(val, time.Minute)
	case "frontlight.manage":
		c.Frontlight.Manage, err = strconv.ParseBool(val)
	case "frontlight.night_level":
		c.Frontlight.NightLevel, err = percent(val)
	case "frontlight.touch_level":
		c.Frontlight.TouchLevel, err = percent(val)
	case "frontlight.touch_time":
		c.Frontlight.TouchTime, err = time.ParseDuration(val)
	case "frontlight.night_when_off":
		c.Frontlight.NightWhenOff = strings.Fields(strings.ReplaceAll(val, ",", " "))
	case "frontlight.sun":
		c.Frontlight.Sun = val
	default:
		return fmt.Errorf("unknown setting %q in [%s]", key, section)
	}
	return err
}

func percent(val string) (int, error) {
	v, err := strconv.Atoi(val)
	if err == nil && (v < 0 || v > 100) {
		err = fmt.Errorf("%d is not between 0 and 100", v)
	}
	return v, err
}

// atLeast parses a duration the alarm clock of the device can handle.
func atLeast(val string, low time.Duration) (time.Duration, error) {
	d, err := time.ParseDuration(val)
	if err == nil && (d < low || d > 18*time.Hour) {
		err = fmt.Errorf("%v is not between %v and 18h", d, low)
	}
	return d, err
}
