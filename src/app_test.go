package main

import (
	"encoding/json"
	"errors"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

type fakeOverlay struct {
	changes int
	visible bool
	startup bool
	toggle  Hotkey
	quits   int
}

func (o *fakeOverlay) Changed()             { o.changes++ }
func (o *fakeOverlay) Visible() bool        { return o.visible }
func (o *fakeOverlay) SetVisible(on bool)   { o.visible = on }
func (o *fakeOverlay) Monitors() int        { return 1 }
func (o *fakeOverlay) OpenFolder()          {}
func (o *fakeOverlay) Quit()                { o.quits++ }
func (o *fakeOverlay) SystemFeatures() bool { return true }
func (o *fakeOverlay) Problem() string      { return "" }
func (o *fakeOverlay) Window(string, int)   {}

func (o *fakeOverlay) ApplyHotkeys(toggle, _ Hotkey) error {
	if toggle.Label == "taken" {
		return errors.New("taken")
	}
	o.toggle = toggle
	return nil
}
func (o *fakeOverlay) StartsWithWindows() bool { return o.startup }
func (o *fakeOverlay) SetStartWithWindows(on bool) error {
	o.startup = on
	return nil
}

func newStore(t *testing.T) (*Store, *fakeOverlay, string) {
	t.Helper()
	dir := t.TempDir()
	overlay := &fakeOverlay{visible: true}
	store, err := OpenStore(dir, overlay)
	if err != nil {
		t.Fatal(err)
	}
	return store, overlay, dir
}

func TestPresetsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, preset := range presets {
		if seen[preset.ID] {
			t.Errorf("duplicate preset %s", preset.ID)
		}
		seen[preset.ID] = true
		if !slices.Contains(categories, preset.Category) {
			t.Errorf("%s has unknown category %q", preset.ID, preset.Category)
		}
		data, _ := json.Marshal(preset.Config)
		if _, err := ParseConfig(data); err != nil {
			t.Errorf("%s: %v", preset.ID, err)
		}
	}
}

func TestFirstRunKeepsExistingCrosshair(t *testing.T) {
	dir := t.TempDir()
	existing := DefaultConfig()
	existing.Color, existing.Gap = "#123456", 9
	if err := SaveConfig(filepath.Join(dir, configName), existing); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir, &fakeOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	state := store.State()
	if len(state.Crosshairs) != 1 || !reflect.DeepEqual(state.Crosshairs[0].Config, existing) {
		t.Fatalf("crosshairs = %+v", state.Crosshairs)
	}
	if state.Active != state.Crosshairs[0].ID || !slices.Equal(state.Recent, []string{state.Active}) {
		t.Errorf("active %q, recent %v", state.Active, state.Recent)
	}
}

func TestUseAppliesCrosshairAndKeepsItsPlace(t *testing.T) {
	store, overlay, dir := newStore(t)
	path := filepath.Join(dir, configName)
	offset, screen := 25, 1
	if err := store.SetOverlay(OverlayUpdate{OffsetX: &offset, Screen: &screen}); err != nil {
		t.Fatal(err)
	}
	before := overlay.changes
	if err := store.Use("red-dot"); err != nil {
		t.Fatal(err)
	}
	if overlay.changes != before+1 {
		t.Error("overlay wasn't told about the change")
	}
	saved, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Color != "#FF2020" || saved.DotSize != 4 || saved.OffsetX != 25 || saved.Screen != 1 {
		t.Errorf("saved = %+v", saved)
	}
}

func TestRecentKeepsLastTenWithoutRepeats(t *testing.T) {
	store, _, _ := newStore(t)
	for _, preset := range presets {
		if err := store.Use(preset.ID); err != nil {
			t.Fatal(err)
		}
	}
	first := presets[len(presets)-5].ID
	if err := store.Use(first); err != nil {
		t.Fatal(err)
	}
	recent := store.State().Recent
	if len(recent) != maxRecent || recent[0] != first {
		t.Fatalf("recent = %v", recent)
	}
	counts := map[string]int{}
	for _, id := range recent {
		if counts[id]++; counts[id] > 1 {
			t.Errorf("%s is repeated", id)
		}
	}
}

func TestDeleteRemovesEveryReference(t *testing.T) {
	store, _, _ := newStore(t)
	id, err := store.Save("", "Temporary", DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	store.Use(id)
	store.SetFavorite(id, true)
	if err := store.Delete(id); err != nil {
		t.Fatal(err)
	}
	state := store.State()
	if slices.Contains(state.Recent, id) || slices.Contains(state.Favorites, id) || state.Active == id {
		t.Errorf("still referenced: %+v", state)
	}
	if err := store.Delete("classic-green"); err == nil {
		t.Error("deleted a built-in preset")
	}
}

func TestEditingCrosshairInUseUpdatesOverlay(t *testing.T) {
	store, _, dir := newStore(t)
	id := store.State().Active
	changed := DefaultConfig()
	changed.Gap = 17
	if _, err := store.Save(id, "  Renamed  ", changed); err != nil {
		t.Fatal(err)
	}
	saved, _ := LoadConfig(filepath.Join(dir, configName))
	if saved.Gap != 17 {
		t.Errorf("gap = %d", saved.Gap)
	}
	if name := store.State().Crosshairs[0].Name; name != "Renamed" {
		t.Errorf("name = %q", name)
	}
}

func TestThemeIsValidatedAndSurvivesRestart(t *testing.T) {
	store, overlay, dir := newStore(t)
	if err := store.SetTheme(Theme{Main: "nope", Accent: "#FFFFFF"}); err == nil {
		t.Error("accepted a bad colour")
	}
	want := Theme{Main: "#FFFFFF", Accent: "#0044CC"}
	if err := store.SetTheme(want); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(dir, overlay)
	if err != nil {
		t.Fatal(err)
	}
	state := reopened.State()
	if state.Theme != want {
		t.Errorf("theme = %+v", state.Theme)
	}
	if !strings.Contains(state.ThemeCSS, "--ink:"+darkText) || !strings.Contains(state.ThemeCSS, "--on-accent:"+lightText) {
		t.Errorf("text isn't readable on these colours: %s", state.ThemeCSS)
	}
}

func TestOldDefaultColorsMoveToNewLook(t *testing.T) {
	for _, test := range []struct {
		saved, want Theme
	}{
		{earlierThemes[0], defaultTheme},
		{earlierThemes[1], defaultTheme},
		{earlierThemes[2], defaultTheme},
		{Theme{Main: "#112233", Accent: "#F4D35E"}, Theme{Main: "#112233", Accent: "#F4D35E"}},
	} {
		dir := t.TempDir()
		saved, _ := json.Marshal(Library{Theme: test.saved})
		if err := os.WriteFile(filepath.Join(dir, libraryName), saved, 0o644); err != nil {
			t.Fatal(err)
		}
		store, err := OpenStore(dir, &fakeOverlay{})
		if err != nil {
			t.Fatal(err)
		}
		if got := store.State().Theme; got != test.want {
			t.Errorf("saved %+v became %+v", test.saved, got)
		}
	}
}

func TestSavedCrosshairsSurviveTheUpdate(t *testing.T) {
	dir := t.TempDir()
	saved := Library{
		Theme:      earlierThemes[2],
		Crosshairs: []Crosshair{{ID: "abc", Name: "Mine", Config: DefaultConfig()}},
		Recent:     []string{"abc", "red-dot"},
		Favorites:  []string{"red-dot"},
		Active:     "abc",
	}
	data, _ := json.Marshal(saved)
	if err := os.WriteFile(filepath.Join(dir, libraryName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir, &fakeOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	state := store.State()
	if len(state.Crosshairs) != 1 || state.Active != "abc" || len(state.Recent) != 2 || len(state.Favorites) != 1 {
		t.Errorf("state = %+v", state)
	}
}

func TestOldFilesMoveToTheDataFolder(t *testing.T) {
	app, data := t.TempDir(), t.TempDir()
	for _, name := range []string{configName, libraryName, logoName} {
		if err := os.WriteFile(filepath.Join(app, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := CopyOldData(app, data); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{configName, libraryName, logoName} {
		if copied, err := os.ReadFile(filepath.Join(data, name)); err != nil || string(copied) != name {
			t.Errorf("%s: %q, %v", name, copied, err)
		}
		if _, err := os.Stat(filepath.Join(app, name)); err != nil {
			t.Errorf("%s: original should stay: %v", name, err)
		}
	}
	// Once the data folder is in use, it's never overwritten.
	os.WriteFile(filepath.Join(app, libraryName), []byte("newer"), 0o644)
	if err := CopyOldData(app, data); err != nil {
		t.Fatal(err)
	}
	if kept, _ := os.ReadFile(filepath.Join(data, libraryName)); string(kept) != libraryName {
		t.Errorf("data folder was overwritten: %q", kept)
	}
}

func TestLogStartsAgainWhenLarge(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, logName)
	if err := os.WriteFile(path, make([]byte, maxLogSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := StartLog(folder)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	defer log.SetOutput(os.Stderr)
	if info, _ := os.Stat(path); info.Size() != 0 {
		t.Errorf("log size = %d", info.Size())
	}
}

func TestTrayIconLoads(t *testing.T) {
	img, err := appIcon()
	if err != nil {
		t.Fatal(err)
	}
	if img.Side != 64 || len(img.Pix) != 64*64*4 {
		t.Fatalf("side = %d", img.Side)
	}
	if blue, red, alpha := img.at(32, 7)[0], img.at(32, 7)[2], img.at(32, 7)[3]; alpha < 200 || red < 150 || blue > 80 {
		t.Errorf("expected the red reticle, got %v", img.at(32, 7))
	}
}

func TestDefaultColorsAreBlackAndRed(t *testing.T) {
	store, _, _ := newStore(t)
	state := store.State()
	if state.Theme.Main != "#000000" || state.Theme.Accent != "#FF0000" {
		t.Errorf("theme = %+v", state.Theme)
	}
	if !strings.Contains(state.ThemeCSS, "--on-accent:"+lightText) {
		t.Errorf("gradient text: %s", state.ThemeCSS)
	}
}

func pointer[T any](value T) *T { return &value }

func TestFontAndTextSizeReachThePage(t *testing.T) {
	store, _, dir := newStore(t)
	if err := store.SetOptions(OptionsUpdate{Font: pointer("Georgia"), TextSize: pointer(125)}); err != nil {
		t.Fatal(err)
	}
	css := store.State().ThemeCSS
	if !strings.Contains(css, `--font:"Georgia"`) || !strings.Contains(css, "zoom:125%") {
		t.Errorf("css = %s", css)
	}
	reopened, err := OpenStore(dir, &fakeOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Options().Font != "Georgia" {
		t.Error("font wasn't saved")
	}
	if err := store.SetOptions(OptionsUpdate{Font: pointer("Poppins"), TextSize: pointer(100)}); err != nil {
		t.Fatal(err)
	}
	if css := store.State().ThemeCSS; strings.Contains(css, "--font") || strings.Contains(css, "zoom") {
		t.Errorf("defaults should add nothing: %s", css)
	}
}

func TestOptionsRejectBadValues(t *testing.T) {
	store, _, _ := newStore(t)
	for _, bad := range []OptionsUpdate{
		{Font: pointer(`x";}body{display:none`)},
		{TextSize: pointer(500)},
		{ToggleKey: &Hotkey{Key: 9999}},
		{ToggleKey: &Hotkey{Key: 119, Modifiers: 99}},
	} {
		if err := store.SetOptions(bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestShortcuts(t *testing.T) {
	store, overlay, _ := newStore(t)
	f8 := Hotkey{Key: 119, Label: "F8"}
	if err := store.SetOptions(OptionsUpdate{ToggleKey: &f8}); err != nil {
		t.Fatal(err)
	}
	if overlay.toggle != f8 {
		t.Errorf("overlay got %+v", overlay.toggle)
	}
	if err := store.SetOptions(OptionsUpdate{NextKey: &f8}); err == nil {
		t.Error("allowed one shortcut for two actions")
	}
	if err := store.SetOptions(OptionsUpdate{ToggleKey: &Hotkey{Key: 120, Label: "taken"}}); err == nil {
		t.Error("should report a shortcut that's taken")
	}
	if got := store.Options().ToggleKey; got != f8 || overlay.toggle != f8 {
		t.Errorf("a failed change should keep the old shortcut, got %+v and %+v", got, overlay.toggle)
	}
}

func TestNextFavoriteCycles(t *testing.T) {
	store, _, _ := newStore(t)
	if err := store.NextFavorite(); err != nil {
		t.Fatal(err)
	}
	store.SetFavorite("red-dot", true)
	store.SetFavorite("hot-pink", true)
	for _, want := range []string{"red-dot", "hot-pink", "red-dot"} {
		if err := store.NextFavorite(); err != nil {
			t.Fatal(err)
		}
		if active := store.State().Active; active != want {
			t.Errorf("active = %s, want %s", active, want)
		}
	}
}

func TestRestoreDefaultsKeepsCrosshairs(t *testing.T) {
	store, overlay, dir := newStore(t)
	id, _ := store.Save("", "Keep me", DefaultConfig())
	store.SetFavorite(id, true)
	store.SetTheme(Theme{Main: "#112233", Accent: "#445566"})
	store.SetOptions(OptionsUpdate{Font: pointer("Arial"), StartInTray: pointer(true), StartWithWindows: pointer(true), ToggleKey: &Hotkey{Key: 119}})
	offset := 40
	store.SetOverlay(OverlayUpdate{OffsetX: &offset})
	if err := store.RestoreDefaults(); err != nil {
		t.Fatal(err)
	}
	state := store.State()
	saved, _ := LoadConfig(filepath.Join(dir, configName))
	if state.Theme != defaultTheme || state.Options != defaultOptions || overlay.startup || saved.OffsetX != 0 || overlay.toggle.Key != 0 {
		t.Errorf("state = %+v, offset %d", state, saved.OffsetX)
	}
	if len(state.Crosshairs) != 2 || len(state.Favorites) != 1 {
		t.Errorf("crosshairs were lost: %+v", state)
	}
}

func TestStateStaysSmall(t *testing.T) {
	store, _, _ := newStore(t)
	data, _ := json.Marshal(store.State())
	if len(data) > 1500 {
		t.Errorf("state is %d bytes", len(data))
	}
}

func TestOlderPictureCrosshairsStillLoad(t *testing.T) {
	dir := t.TempDir()
	saved := `{"theme":{"main":"#000000","accent":"#FF0000"},"active":"duck","recent":["duck","red-dot"],"favorites":["duck"],
		"crosshairs":[{"id":"abc","name":"Duck target copy","config":{"color":"#FFFFFF","opacity":1,"length":8,"gap":4,"thickness":2,"outline":0,"outline_color":"#000000","image":"duck","size":120}}]}`
	if err := os.WriteFile(filepath.Join(dir, libraryName), []byte(saved), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, configName), []byte(`{"color":"#FFFFFF","image":"duck","size":120}`), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir, &fakeOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	state := store.State()
	if state.Active != "" || !slices.Equal(state.Recent, []string{"red-dot"}) || len(state.Favorites) != 0 {
		t.Errorf("removed presets should drop out: %+v", state)
	}
	if len(state.Crosshairs) != 1 || filled(Render(state.Crosshairs[0].Config)) == 0 {
		t.Errorf("saved copy should still draw: %+v", state.Crosshairs)
	}
	if _, err := LoadConfig(filepath.Join(dir, configName)); err != nil {
		t.Errorf("old settings file: %v", err)
	}
}

func TestQuitRoute(t *testing.T) {
	signed, server := signedIn(t)
	stranger := signed
	stranger.cookie = nil
	headers := map[string]string{"Content-Type": "application/json"}
	overlay := server.store.overlay.(*fakeOverlay)
	if status := stranger.send("POST", "/api/quit", "{}", headers).StatusCode; status != http.StatusForbidden || overlay.quits != 0 {
		t.Errorf("outsiders must not be able to quit: %d", status)
	}
	if status := signed.send("POST", "/api/quit", "{}", headers).StatusCode; status != http.StatusOK || overlay.quits != 1 {
		t.Errorf("quit: status %d, %d quits", status, overlay.quits)
	}
}

func TestGradientTextReadsAcrossTheFade(t *testing.T) {
	for _, theme := range []Theme{
		defaultTheme,
		{Main: "#FFFFFF", Accent: "#0044CC"},
		{Main: "#000000", Accent: "#FFFF00"},
		{Main: "#FFFFFF", Accent: "#FF0000"},
		{Main: "#102040", Accent: "#00FF88"},
	} {
		text := textOnGradient(theme)
		middle := mixColors(theme.Accent, theme.Main, gradientTextReach)
		if contrast(theme.Accent, text) < 2 || contrast(middle, text) < 2 {
			t.Errorf("%+v: %s is hard to read on %s or %s", theme, text, theme.Accent, middle)
		}
	}
	if mixed := mixColors("#FF0000", "#000000", 0.5); mixed != "#800000" {
		t.Errorf("mixed = %s", mixed)
	}
}

func TestNewInstallsStartWithTheDefaultLook(t *testing.T) {
	store, _, _ := newStore(t)
	options := store.Options()
	if store.lib.Theme != (Theme{Main: "#000000", Accent: "#FF0000"}) {
		t.Errorf("colors = %+v", store.lib.Theme)
	}
	if options.Font != "Bahnschrift" || options.Background != "minimal" || options.TextSize != 0 || options.NoAnimations {
		t.Errorf("options = %+v", options)
	}
	if css := store.State().ThemeCSS; !strings.Contains(css, `"Bahnschrift",Poppins`) {
		t.Errorf("theme = %s", css)
	}
}

func TestGuideStaysClosed(t *testing.T) {
	store, overlay, dir := newStore(t)
	hide := true
	if err := store.SetOptions(OptionsUpdate{HideGuide: &hide}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(dir, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Options().HideGuide {
		t.Error("the guide came back after restarting")
	}
	if err := reopened.RestoreDefaults(); err != nil {
		t.Fatal(err)
	}
	if !reopened.Options().HideGuide {
		t.Error("restoring defaults brought the guide back")
	}
}

func TestDraftsSurviveARestart(t *testing.T) {
	store, overlay, dir := newStore(t)
	shape := DefaultConfig()
	shape.Strokes = []Stroke{{Color: "#FF0000", Width: 2, Points: []float64{1, 2, 3, 4}}}
	if err := store.SetDraft(&Crosshair{Name: "Unfinished", Config: shape}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(dir, overlay)
	if err != nil {
		t.Fatal(err)
	}
	draft := reopened.State().Draft
	if draft == nil || draft.Name != "Unfinished" || len(draft.Config.Strokes) != 1 {
		t.Fatalf("draft = %+v", draft)
	}
	if err := reopened.SetDraft(nil); err != nil {
		t.Fatal(err)
	}
	if reopened.State().Draft != nil {
		t.Error("the draft wasn't cleared")
	}
}

func TestDamagedDraftIsDropped(t *testing.T) {
	store, overlay, dir := newStore(t)
	broken := DefaultConfig()
	broken.Strokes = []Stroke{{Color: "#FF0000", Width: 2, Points: []float64{1}}}
	if err := store.SetDraft(&Crosshair{Name: "Broken", Config: broken}); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenStore(dir, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.State().Draft != nil {
		t.Error("a damaged draft was kept")
	}
}

func TestTitleBarMatchesTheTheme(t *testing.T) {
	theme := Theme{Main: "#102040", Accent: "#FF0000"}
	if got := titleColor(theme, Options{Background: "minimal"}); got != "#102040" {
		t.Errorf("minimal = %s", got)
	}
	if got := titleColor(theme, Options{}); got != "#102040" {
		t.Errorf("default = %s", got)
	}
	if got := titleColor(theme, Options{Background: "fade"}); got == "#102040" || got == "#FF0000" {
		t.Errorf("fade = %s", got)
	}
}

func TestBackgroundThemes(t *testing.T) {
	store, _, _ := newStore(t)
	for _, name := range []string{"fade", "minimal", ""} {
		if err := store.SetOptions(OptionsUpdate{Background: &name}); err != nil {
			t.Errorf("%q: %v", name, err)
		}
		if store.Options().Background != name {
			t.Errorf("%q wasn't saved", name)
		}
	}
	hide := true
	if err := store.SetOptions(OptionsUpdate{HideGuide: &hide}); err != nil || !store.Options().HideGuide {
		t.Errorf("the guide wasn't dismissed: %v", err)
	}
	bad := "\"><script>"
	if store.SetOptions(OptionsUpdate{Background: &bad}) == nil {
		t.Error("an unknown theme was accepted")
	}
}

func TestDamagedLibraryIsKeptAside(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, libraryName), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir, &fakeOverlay{})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.State().Crosshairs) != 1 {
		t.Error("expected a fresh library")
	}
	if _, err := os.Stat(filepath.Join(dir, libraryName+".broken")); err != nil {
		t.Error("damaged file wasn't kept")
	}
}

type client struct {
	t      *testing.T
	base   string
	host   string
	cookie *http.Cookie
}

func (c client) send(method, path, body string, headers map[string]string) *http.Response {
	c.t.Helper()
	request, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	request.Host = c.host
	if c.cookie != nil {
		request.AddCookie(c.cookie)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	noRedirect := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := noRedirect.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	c.t.Cleanup(func() { response.Body.Close() })
	return response
}

func signedIn(t *testing.T) (client, *Server) {
	t.Helper()
	store, _, _ := newStore(t)
	server, err := StartServer(store)
	if err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse(server.URL())
	if err != nil {
		t.Fatal(err)
	}
	guest := client{t: t, base: "http://" + address.Host, host: address.Host}
	response := guest.send("GET", "/?t="+server.token, "", nil)
	if response.StatusCode != http.StatusFound || len(response.Cookies()) != 1 {
		t.Fatalf("sign-in: %d", response.StatusCode)
	}
	guest.cookie = response.Cookies()[0]
	if guest.cookie.SameSite != http.SameSiteStrictMode || !guest.cookie.HttpOnly {
		t.Error("session cookie isn't locked down")
	}
	return guest, server
}

func TestServerTurnsAwayOutsiders(t *testing.T) {
	signed, server := signedIn(t)
	stranger := signed
	stranger.cookie = nil
	json := map[string]string{"Content-Type": "application/json"}
	checks := []struct {
		name string
		got  int
	}{
		{"no session", stranger.send("GET", "/api/state", "", nil).StatusCode},
		{"wrong secret", stranger.send("GET", "/?t=wrong", "", nil).StatusCode},
		{"changing nothing without a session", stranger.send("POST", "/api/use", `{"id":"red-dot"}`, json).StatusCode},
		{"plain form post", signed.send("POST", "/api/use", `{"id":"red-dot"}`, map[string]string{"Content-Type": "text/plain"}).StatusCode},
		{"another website", signed.send("POST", "/api/use", `{"id":"red-dot"}`, map[string]string{
			"Content-Type": "application/json", "Origin": "https://example.com"}).StatusCode},
	}
	for _, check := range checks {
		if check.got != http.StatusForbidden {
			t.Errorf("%s: status %d", check.name, check.got)
		}
	}
	rebound := signed
	rebound.host = "evil.example.com"
	if status := rebound.send("GET", "/api/state", "", nil).StatusCode; status != http.StatusForbidden {
		t.Errorf("wrong host: status %d", status)
	}
	if server.store.State().Active == "red-dot" {
		t.Error("a rejected request changed the crosshair")
	}
}

func TestServerPagesAndActions(t *testing.T) {
	signed, server := signedIn(t)
	page := signed.send("GET", "/", "", nil)
	if page.StatusCode != http.StatusOK {
		t.Fatalf("page: %d", page.StatusCode)
	}
	body := make([]byte, 4096)
	count, _ := page.Body.Read(body)
	if !strings.Contains(string(body[:count]), "--main:"+defaultTheme.Main) {
		t.Error("page doesn't include the theme")
	}
	if page.Header.Get("Content-Security-Policy") == "" {
		t.Error("missing content security policy")
	}
	for _, path := range []string{"/app.js", "/editor.js", "/settings.js", "/app.css", "/icon.png", "/api/catalog", "/fonts/poppins-400.woff", "/fonts/poppins-500.woff", "/fonts/poppins-700.woff"} {
		if status := signed.send("GET", path, "", nil).StatusCode; status != http.StatusOK {
			t.Errorf("%s: %d", path, status)
		}
	}
	if signed.send("GET", "/logo.png", "", nil).StatusCode != http.StatusNotFound || server.store.State().CustomLogo {
		t.Error("the built-in logo should show when there's no logo.png")
	}
	if err := os.WriteFile(server.store.logoPath, []byte("my logo"), 0o644); err != nil {
		t.Fatal(err)
	}
	replaced := make([]byte, 16)
	count, _ = signed.send("GET", "/logo.png", "", nil).Body.Read(replaced)
	if string(replaced[:count]) != "my logo" {
		t.Errorf("logo.png next to the app wasn't used: %q", replaced[:count])
	}
	if !server.store.State().CustomLogo {
		t.Error("the app wasn't told about logo.png")
	}

	headers := map[string]string{"Content-Type": "application/json"}
	response := signed.send("POST", "/api/crosshair", `{"name":"Mine","config":{"gap":3}}`, headers)
	var reply struct {
		ID    string `json:"id"`
		State State  `json:"state"`
	}
	if err := json.NewDecoder(response.Body).Decode(&reply); err != nil {
		t.Fatal(err)
	}
	if reply.ID == "" || len(reply.State.Crosshairs) != 2 || reply.State.Crosshairs[1].Config.Gap != 3 {
		t.Errorf("reply = %+v", reply)
	}
	if status := signed.send("POST", "/api/crosshair", `{"config":{"color":"bad"}}`, headers).StatusCode; status != http.StatusBadRequest {
		t.Errorf("bad colour: %d", status)
	}
	if status := signed.send("POST", "/api/use", `{"id":"missing"}`, headers).StatusCode; status != http.StatusBadRequest {
		t.Errorf("missing crosshair: %d", status)
	}
	if status := signed.send("POST", "/api/overlay", `{"visible":false}`, headers).StatusCode; status != http.StatusOK {
		t.Errorf("overlay: %d", status)
	}
	if server.store.State().Overlay.Visible {
		t.Error("overlay should be hidden")
	}
}

func TestPreviewMatchesOverlayDrawing(t *testing.T) {
	signed, _ := signedIn(t)
	query := url.QueryEscape(`{"color":"#FF0000","length":8,"gap":4,"thickness":2,"outline":1}`)
	response := signed.send("GET", "/render.png?scale=2&c="+query, "", nil)
	decoded, err := png.Decode(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if size := decoded.Bounds().Dx(); size != 54 {
		t.Fatalf("size = %d, want 54", size)
	}
	red, green, blue, alpha := decoded.At(43, 27).RGBA()
	if red>>8 != 255 || green != 0 || blue != 0 || alpha>>8 != 255 {
		t.Errorf("arm pixel = %d %d %d %d", red, green, blue, alpha)
	}
	if status := signed.send("GET", "/render.png?c=broken", "", nil).StatusCode; status != http.StatusBadRequest {
		t.Errorf("bad config: %d", status)
	}
}

func TestReadableTextOnAnyColour(t *testing.T) {
	for _, background := range []string{"#000000", "#FFFFFF", "#332A2B", "#F4D35E", "#777777", "#0044CC", "#FF0000"} {
		if ratio := contrast(background, readableOn(background)); ratio < 4 {
			t.Errorf("%s: contrast %.2f", background, ratio)
		}
	}
}
