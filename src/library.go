// The saved data behind the app window: your crosshairs, recents, favorites and
// app colours. Nothing here touches Windows, so `go test` works on any system.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

const (
	appName     = "Freezy Aim" // Shown in the sidebar, title bar and tray.
	libraryName = "crosshair_library.json"
	logoName    = "logo.png" // A file with this name in the data folder replaces the built-in logo.
	logName     = "freezy-aim.log"
	maxLogSize  = 1 << 20
	maxRecent   = 10
	maxName     = 40

	lightText = "#F4F6FF"
	darkText  = "#0B1020"

	// gradientTextReach is how far across a gradient button its label usually runs.
	gradientTextReach = 0.55
)

// version is replaced at build time for releases.
var version = "1.0"

var (
	defaultTheme = Theme{Main: "#000000", Accent: "#FF0000"}

	// fontChoices are offered in Settings. Poppins is built in, and the rest are used
	// when the system has them. The first is the default, and the sidebar's app name always uses it.
	fontChoices = []string{
		"Poppins", "Segoe UI", "Bahnschrift", "Arial", "Calibri", "Consolas",
		"Georgia", "Tahoma", "Trebuchet MS", "Verdana",
	}

	// defaultOptions is the look a new install starts with. Bahnschrift comes with
	// Windows. Systems without it fall back to the built-in Poppins.
	defaultOptions = Options{Font: "Bahnschrift", Background: "minimal"}

	// backgrounds are the themes offered in Settings. Empty is the Default theme.
	backgrounds = []string{"", "fade", "minimal"}

	// textSizes are the percentages offered in Settings.
	textSizes = []int{90, 100, 110, 125}

	// earlierThemes were defaults in past versions. Anyone still on one moves to the
	// current default, while colours people chose themselves are left alone.
	earlierThemes = []Theme{
		{Main: "#332A2B", Accent: "#F4D35E"},
		{Main: "#070B16", Accent: "#18A8F5"},
		{Main: "#070B16", Accent: "#149EF2"},
	}
)

var errMissing = errors.New("That crosshair no longer exists.")

type Theme struct {
	Main   string `json:"main"`
	Accent string `json:"accent"`
}

type Crosshair struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"` // Used by built-in presets.
	Config   Config `json:"config"`
}

// Library is what crosshair_library.json holds.
type Library struct {
	Theme      Theme       `json:"theme"`
	Crosshairs []Crosshair `json:"crosshairs"`
	Recent     []string    `json:"recent"`
	Favorites  []string    `json:"favorites"`
	Active     string      `json:"active"`
	Options    Options     `json:"options"`
	Draft      *Crosshair  `json:"draft,omitempty"` // A drawing that hasn't been saved yet.
}

// Hotkey is a global keyboard shortcut.
type Hotkey struct {
	Key       int    `json:"key"`       // Windows virtual-key code. 0 means no shortcut.
	Modifiers int    `json:"modifiers"` // Alt 1, Ctrl 2, Shift 4.
	Label     string `json:"label"`     // What the app shows, such as "Ctrl+F8".
}

// Options holds the choices in Settings. Empty values mean the default.
type Options struct {
	Font         string `json:"font,omitempty"`
	TextSize     int    `json:"text_size,omitempty"` // Percent.
	Background   string `json:"background,omitempty"`
	NoAnimations bool   `json:"no_animations,omitempty"`
	StartInTray  bool   `json:"start_in_tray,omitempty"`
	HideGuide    bool   `json:"hide_guide,omitempty"` // The welcome tips were dismissed.
	KeepTitleBar bool   `json:"keep_title_bar,omitempty"`
	ToggleKey    Hotkey `json:"toggle_key"`
	NextKey      Hotkey `json:"next_key"`
}

// OptionsUpdate changes only the fields that are set.
type OptionsUpdate struct {
	Font             *string `json:"font"`
	TextSize         *int    `json:"text_size"`
	Background       *string `json:"background"`
	NoAnimations     *bool   `json:"no_animations"`
	StartInTray      *bool   `json:"start_in_tray"`
	HideGuide        *bool   `json:"hide_guide"`
	KeepTitleBar     *bool   `json:"keep_title_bar"`
	StartWithWindows *bool   `json:"start_with_windows"`
	ToggleKey        *Hotkey `json:"toggle_key"`
	NextKey          *Hotkey `json:"next_key"`
}

// Overlay is how the app window talks to the on-screen crosshair.
type Overlay interface {
	Changed() // The settings file was rewritten, so apply it now.
	Visible() bool
	SetVisible(on bool)
	Monitors() int
	OpenFolder() // Shows the data folder.
	Quit()       // Closes the whole app.
	ApplyHotkeys(toggle, next Hotkey) error
	StartsWithWindows() bool
	SetStartWithWindows(on bool) error
	SystemFeatures() bool                // Whether shortcuts and starting with the system work here.
	Problem() string                     // Anything stopping the crosshair showing, for the app window.
	Window(action string, barHeight int) // Controls the app window where that's possible.
}

type OverlayState struct {
	Visible  bool `json:"visible"`
	Monitors int  `json:"monitors"`

	StartsWithWindows bool   `json:"starts_with_windows"`
	Problem           string `json:"problem,omitempty"`

	Screen  int `json:"screen"`
	OffsetX int `json:"offset_x"`
	OffsetY int `json:"offset_y"`
}

// Catalog holds what never changes while the app runs. It's sent once.
type Catalog struct {
	AppName        string      `json:"app_name"`
	Version        string      `json:"version"`
	DefaultTheme   Theme       `json:"default_theme"`
	Presets        []Crosshair `json:"presets"`
	Categories     []string    `json:"categories"`
	Bases          []Crosshair `json:"bases"`
	BaseGroups     []string    `json:"base_groups"`
	SystemFeatures bool        `json:"system_features"`
	Fonts          []string    `json:"fonts"`
}

func NewCatalog(systemFeatures bool) Catalog {
	return Catalog{
		AppName:        appName,
		Version:        version,
		DefaultTheme:   defaultTheme,
		Presets:        presets,
		Categories:     categories,
		Bases:          bases,
		BaseGroups:     baseGroups,
		SystemFeatures: systemFeatures,
		Fonts:          fontChoices,
	}
}

// State is what changes as you use the app.
type State struct {
	Theme         Theme        `json:"theme"`
	ThemeCSS      string       `json:"theme_css"`
	ThemeContrast float64      `json:"theme_contrast"`
	TitleColor    string       `json:"title_color"`
	Options       Options      `json:"options"`
	Crosshairs    []Crosshair  `json:"crosshairs"`
	Recent        []string     `json:"recent"`
	Favorites     []string     `json:"favorites"`
	Active        string       `json:"active"`
	CustomLogo    bool         `json:"custom_logo,omitempty"` // A logo.png replaces the built-in logo.
	Draft         *Crosshair   `json:"draft,omitempty"`
	Overlay       OverlayState `json:"overlay"`
}

type Store struct {
	mu          sync.Mutex
	lib         Library
	configPath  string
	libraryPath string
	logoPath    string
	overlay     Overlay
}

// DataFolder returns where settings live, creating it if needed. If that isn't
// possible, it falls back to the folder the app is in.
func DataFolder(appFolder string) string {
	base, err := os.UserConfigDir()
	if err != nil {
		return appFolder
	}
	folder := filepath.Join(base, appName)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return appFolder
	}
	return folder
}

// CopyOldData brings over files that earlier versions kept next to the app. It
// only runs once, and it leaves the originals where they are.
func CopyOldData(appFolder, dataFolder string) error {
	if appFolder == dataFolder {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dataFolder, libraryName)); err == nil {
		return nil
	}
	for _, name := range []string{configName, libraryName, logoName} {
		data, err := os.ReadFile(filepath.Join(appFolder, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dataFolder, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// StartLog sends problems to a file in the data folder, starting it again if it
// has grown too large. The log never contains anything but the app's own errors.
func StartLog(folder string) (io.Closer, error) {
	path := filepath.Join(folder, logName)
	if info, err := os.Stat(path); err == nil && info.Size() > maxLogSize {
		os.Remove(path)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(file)
	return file, nil
}

func OpenStore(dir string, overlay Overlay) (*Store, error) {
	s := &Store{
		configPath:  filepath.Join(dir, configName),
		libraryPath: filepath.Join(dir, libraryName),
		logoPath:    filepath.Join(dir, logoName),
		overlay:     overlay,
	}
	current := s.settingsFile()
	data, err := os.ReadFile(s.libraryPath)
	switch {
	case err == nil:
		err = json.Unmarshal(bytes.TrimPrefix(data, []byte(byteOrder)), &s.lib)
		if err != nil {
			// Keep the damaged file so nothing in it is lost, then start fresh.
			os.Rename(s.libraryPath, s.libraryPath+".broken")
			s.lib = firstLibrary(current)
		}
	case errors.Is(err, os.ErrNotExist):
		s.lib = firstLibrary(current)
	default:
		return nil, err
	}
	s.tidy()
	return s, s.commit()
}

// firstLibrary keeps the crosshair already in the settings file as the first one.
func firstLibrary(current Config) Library {
	first := Crosshair{ID: newID(), Name: "My crosshair", Config: current}
	return Library{
		Theme:      defaultTheme,
		Options:    defaultOptions,
		Crosshairs: []Crosshair{first},
		Recent:     []string{first.ID},
		Active:     first.ID,
	}
}

func newID() string {
	bytes := make([]byte, 6)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// tidy repairs anything a hand-edited library file could have broken.
func (s *Store) tidy() {
	if slices.Contains(earlierThemes, s.lib.Theme) {
		s.lib.Theme = defaultTheme
	}
	if _, err := parseColor(s.lib.Theme.Main); err != nil {
		s.lib.Theme.Main = defaultTheme.Main
	}
	if _, err := parseColor(s.lib.Theme.Accent); err != nil {
		s.lib.Theme.Accent = defaultTheme.Accent
	}
	if s.lib.Crosshairs == nil {
		s.lib.Crosshairs = []Crosshair{}
	}
	s.lib.Recent = s.existing(s.lib.Recent)
	s.lib.Favorites = s.existing(s.lib.Favorites)
	if len(s.lib.Recent) > maxRecent {
		s.lib.Recent = s.lib.Recent[:maxRecent]
	}
	if _, ok := s.find(s.lib.Active); !ok {
		s.lib.Active = ""
	}
	if !slices.Contains(fontChoices, s.lib.Options.Font) {
		s.lib.Options.Font = ""
	}
	if s.lib.Draft != nil && s.lib.Draft.Config.clean() != nil {
		s.lib.Draft = nil // A damaged draft can't be drawn, so don't offer it.
	}
	if !slices.Contains(backgrounds, s.lib.Options.Background) {
		s.lib.Options.Background = ""
	}
	if !slices.Contains(textSizes, s.lib.Options.TextSize) {
		s.lib.Options.TextSize = 0
	}
}

func (s *Store) existing(ids []string) []string {
	kept := []string{}
	for _, id := range ids {
		if _, ok := s.find(id); ok && !slices.Contains(kept, id) {
			kept = append(kept, id)
		}
	}
	return kept
}

func (s *Store) find(id string) (Crosshair, bool) {
	for _, list := range [][]Crosshair{s.lib.Crosshairs, presets} {
		if index := slices.IndexFunc(list, func(c Crosshair) bool { return c.ID == id }); index >= 0 {
			return list[index], true
		}
	}
	return Crosshair{}, false
}

// ownIndex finds a saved crosshair, leaving out the built-in presets.
func (s *Store) ownIndex(id string) int {
	return slices.IndexFunc(s.lib.Crosshairs, func(c Crosshair) bool { return c.ID == id })
}

func (s *Store) commit() error {
	data, err := json.MarshalIndent(s.lib, "", "  ")
	if err != nil {
		return err
	}
	return writeFileSafely(s.libraryPath, append(data, '\n'))
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	position := s.settingsFile()
	_, logoErr := os.Stat(s.logoPath)
	return State{
		Theme:         s.lib.Theme,
		ThemeCSS:      themeCSS(s.lib.Theme, s.lib.Options),
		ThemeContrast: math.Round(contrast(s.lib.Theme.Main, s.lib.Theme.Accent)*100) / 100,
		TitleColor:    titleColor(s.lib.Theme, s.lib.Options),
		Options:       s.lib.Options,
		Crosshairs:    s.lib.Crosshairs,
		Recent:        s.lib.Recent,
		Favorites:     s.lib.Favorites,
		Active:        s.lib.Active,
		CustomLogo:    logoErr == nil,
		Draft:         s.lib.Draft,
		Overlay: OverlayState{
			Visible:           s.overlay.Visible(),
			Monitors:          s.overlay.Monitors(),
			StartsWithWindows: s.overlay.StartsWithWindows(),
			Problem:           s.overlay.Problem(),
			Screen:            position.Screen,
			OffsetX:           position.OffsetX,
			OffsetY:           position.OffsetY,
		},
	}
}

// settingsFile returns the saved settings, or the defaults if they can't be read.
func (s *Store) settingsFile() Config {
	current, err := LoadConfig(s.configPath)
	if err != nil {
		return DefaultConfig()
	}
	return current
}

// updateSettings rewrites the settings file and tells the overlay right away.
func (s *Store) updateSettings(change func(*Config)) error {
	current := s.settingsFile()
	change(&current)
	if err := SaveConfig(s.configPath, current); err != nil {
		return err
	}
	s.overlay.Changed()
	return nil
}

// draw puts a crosshair on screen without moving it from where it's placed.
func (s *Store) draw(shape Config) error {
	return s.updateSettings(func(current *Config) {
		shape.OffsetX, shape.OffsetY, shape.Screen = current.OffsetX, current.OffsetY, current.Screen
		*current = shape
	})
}

func (s *Store) Use(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.find(id)
	if !ok {
		return errMissing
	}
	if err := s.draw(item.Config); err != nil {
		return err
	}
	s.lib.Active = id
	s.lib.Recent = append([]string{id}, without(s.lib.Recent, id)...)
	if len(s.lib.Recent) > maxRecent {
		s.lib.Recent = s.lib.Recent[:maxRecent]
	}
	return s.commit()
}

func without(ids []string, id string) []string {
	return slices.DeleteFunc(slices.Clone(ids), func(other string) bool { return other == id })
}

func (s *Store) SetFavorite(id string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.find(id); !ok {
		return errMissing
	}
	s.lib.Favorites = without(s.lib.Favorites, id)
	if on {
		s.lib.Favorites = append(s.lib.Favorites, id)
	}
	return s.commit()
}

// Options returns a copy of the current choices.
func (s *Store) Options() Options {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lib.Options
}

func validHotkey(key Hotkey) bool {
	return key.Key == 0 || (key.Key > 0 && key.Key < 255 && key.Modifiers >= 0 && key.Modifiers < 8 && len(key.Label) < 32)
}

func (s *Store) SetOptions(update OptionsUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.lib.Options
	if update.Font != nil {
		if !slices.Contains(fontChoices, *update.Font) {
			return errors.New("That font isn't available.")
		}
		next.Font = *update.Font
	}
	if update.TextSize != nil {
		if !slices.Contains(textSizes, *update.TextSize) {
			return errors.New("That text size isn't available.")
		}
		next.TextSize = *update.TextSize
	}
	if update.Background != nil {
		if !slices.Contains(backgrounds, *update.Background) {
			return errors.New("That theme isn't available.")
		}
		next.Background = *update.Background
	}
	if update.NoAnimations != nil {
		next.NoAnimations = *update.NoAnimations
	}
	if update.KeepTitleBar != nil {
		next.KeepTitleBar = *update.KeepTitleBar
	}
	if update.HideGuide != nil {
		next.HideGuide = *update.HideGuide
	}
	if update.StartInTray != nil {
		next.StartInTray = *update.StartInTray
	}
	if update.ToggleKey != nil {
		next.ToggleKey = *update.ToggleKey
	}
	if update.NextKey != nil {
		next.NextKey = *update.NextKey
	}
	if update.ToggleKey != nil || update.NextKey != nil {
		if !validHotkey(next.ToggleKey) || !validHotkey(next.NextKey) {
			return errors.New("That shortcut can't be used.")
		}
		if next.ToggleKey.Key != 0 && next.ToggleKey.Key == next.NextKey.Key && next.ToggleKey.Modifiers == next.NextKey.Modifiers {
			return errors.New("Each action needs its own shortcut.")
		}
		if err := s.overlay.ApplyHotkeys(next.ToggleKey, next.NextKey); err != nil {
			s.overlay.ApplyHotkeys(s.lib.Options.ToggleKey, s.lib.Options.NextKey)
			return err
		}
	}
	if update.StartWithWindows != nil {
		if err := s.overlay.SetStartWithWindows(*update.StartWithWindows); err != nil {
			return err
		}
	}
	s.lib.Options = next
	return s.commit()
}

// NextFavorite moves to the favorite after the one in use, for the shortcut.
func (s *Store) NextFavorite() error {
	s.mu.Lock()
	favorites := s.lib.Favorites
	active := s.lib.Active
	s.mu.Unlock()
	if len(favorites) == 0 {
		return nil
	}
	return s.Use(favorites[(slices.Index(favorites, active)+1)%len(favorites)])
}

// RestoreDefaults resets Settings. Saved crosshairs and favorites are kept.
func (s *Store) RestoreDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lib.Theme = defaultTheme
	hideGuide := s.lib.Options.HideGuide // Tips you've closed stay closed.
	s.lib.Options = defaultOptions
	s.lib.Options.HideGuide = hideGuide
	s.overlay.ApplyHotkeys(Hotkey{}, Hotkey{})
	s.overlay.SetVisible(true)
	if err := s.overlay.SetStartWithWindows(false); err != nil {
		return err
	}
	if err := s.updateSettings(func(current *Config) {
		current.OffsetX, current.OffsetY, current.Screen = 0, 0, 0
	}); err != nil {
		return err
	}
	return s.commit()
}

func cleanName(name string) string {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) > maxName {
		runes = runes[:maxName]
	}
	if len(runes) == 0 {
		return "Untitled crosshair"
	}
	return string(runes)
}

// Save adds a crosshair, or updates one if it already has an ID. It returns the
// crosshair's ID. Editing the crosshair in use updates the overlay as you go.
func (s *Store) Save(id, name string, shape Config) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		item := Crosshair{ID: newID(), Name: cleanName(name), Config: shape}
		s.lib.Crosshairs = append(s.lib.Crosshairs, item)
		return item.ID, s.commit()
	}
	index := s.ownIndex(id)
	if index < 0 {
		return "", errMissing
	}
	s.lib.Crosshairs[index].Name = cleanName(name)
	s.lib.Crosshairs[index].Config = shape
	if s.lib.Active == id {
		if err := s.draw(shape); err != nil {
			return "", err
		}
	}
	return id, s.commit()
}

// Copy saves a copy of any crosshair, including a preset, so it can be edited.
func (s *Store) Copy(id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source, ok := s.find(id)
	if !ok {
		return "", errMissing
	}
	item := Crosshair{ID: newID(), Name: cleanName(source.Name + " copy"), Config: source.Config}
	s.lib.Crosshairs = append(s.lib.Crosshairs, item)
	return item.ID, s.commit()
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.ownIndex(id)
	if index < 0 {
		return errMissing
	}
	s.lib.Crosshairs = slices.Delete(s.lib.Crosshairs, index, index+1)
	s.tidy()
	return s.commit()
}

// SetDraft keeps a new drawing safe until it's saved. Nil clears it.
func (s *Store) SetDraft(draft *Crosshair) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lib.Draft = draft
	return s.commit()
}

func (s *Store) SetTheme(theme Theme) error {
	for _, color := range []string{theme.Main, theme.Accent} {
		if _, err := parseColor(color); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lib.Theme = theme
	return s.commit()
}

// OverlayUpdate changes only the fields that are set.
type OverlayUpdate struct {
	Visible *bool `json:"visible"`
	Screen  *int  `json:"screen"`
	OffsetX *int  `json:"offset_x"`
	OffsetY *int  `json:"offset_y"`
}

func (s *Store) SetOverlay(update OverlayUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if update.Visible != nil {
		s.overlay.SetVisible(*update.Visible)
	}
	if update.Screen == nil && update.OffsetX == nil && update.OffsetY == nil {
		return nil
	}
	return s.updateSettings(func(current *Config) {
		if update.Screen != nil {
			current.Screen = *update.Screen
		}
		if update.OffsetX != nil {
			current.OffsetX = *update.OffsetX
		}
		if update.OffsetY != nil {
			current.OffsetY = *update.OffsetY
		}
	})
}

// themeCSS turns the app's look into page styles. Text colours are picked for you,
// so words stay readable on any main or accent colour. Fonts and sizes come from
// fixed lists, so nothing from the settings file can reach the page unchecked.
func themeCSS(theme Theme, options Options) string {
	css := fmt.Sprintf(":root{--main:%s;--accent:%s;--ink:%s;--on-accent:%s",
		theme.Main, theme.Accent, readableOn(theme.Main), textOnGradient(theme))
	if options.Font != "" && options.Font != fontChoices[0] {
		css += fmt.Sprintf(`;--font:"%s",Poppins,sans-serif`, options.Font)
	}
	if options.TextSize != 0 && options.TextSize != 100 {
		css += fmt.Sprintf(";zoom:%d%%", options.TextSize)
	}
	return css + "}"
}

// titleColor matches the window's title bar to the top of the app. The Color fade
// theme starts from the accent colour, and the others from the main colour.
func titleColor(theme Theme, options Options) string {
	if options.Background == "fade" {
		return mixColors(theme.Accent, theme.Main, 0.44)
	}
	return theme.Main
}

// mixColors blends two colours, with share being how much of the first to use.
func mixColors(first, second string, share float64) string {
	a, errA := parseColor(first)
	b, errB := parseColor(second)
	if errA != nil || errB != nil {
		return first
	}
	blend := func(channel int) int {
		return int(float64(a[channel])*share + float64(b[channel])*(1-share) + 0.5)
	}
	return fmt.Sprintf("#%02X%02X%02X", blend(2), blend(1), blend(0))
}

// textOnGradient picks the text colour for gradient buttons, which fade from the
// accent colour into the main colour. Text has to read at both ends of the fade.
func textOnGradient(theme Theme) string {
	middle := mixColors(theme.Accent, theme.Main, gradientTextReach)
	worst := func(text string) float64 {
		return min(contrast(theme.Accent, text), contrast(middle, text))
	}
	if worst(lightText) >= 2.5 || worst(lightText) >= worst(darkText) {
		return lightText
	}
	return darkText
}

// readableOn prefers the softer tones and falls back to pure white or black on
// mid-tone backgrounds, where the softer ones are too faint.
func readableOn(background string) string {
	best := ""
	for _, text := range []string{lightText, darkText, "#FFFFFF", "#000000"} {
		if contrast(background, text) >= 4.5 {
			return text
		}
		if best == "" || contrast(background, text) > contrast(background, best) {
			best = text
		}
	}
	return best
}

func luminance(color string) float64 {
	parsed, err := parseColor(color)
	if err != nil {
		return 0
	}
	linear := func(value byte) float64 {
		channel := float64(value) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.0722*linear(parsed[0]) + 0.7152*linear(parsed[1]) + 0.2126*linear(parsed[2])
}

// contrast returns the WCAG contrast ratio, from 1 (identical) to 21.
func contrast(first, second string) float64 {
	a, b := luminance(first), luminance(second)
	return (max(a, b) + 0.05) / (min(a, b) + 0.05)
}
