//go:build !windows

// Freezy Aim for macOS and Linux. The app window and crosshair drawing are shared with
// Windows. A click-through window needs native code on these systems, so a small
// helper, overlay.py, puts the finished crosshair on screen and adds a tray icon.
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

//go:embed overlay.py
var overlayScript []byte

const (
	overlayScriptName = "overlay.py"
	overlayImageName  = "overlay.png"
	overlaySharpName  = "overlay@2x.png" // For Retina and other high-resolution screens.
	overlayStateName  = "overlay.json"
	overlayStatusName = "overlay_status.json"
	trayIconName      = "tray.png"
	instanceName      = "instance.url"
	missingQtExit     = 3 // overlay.py exits with this when PySide6 isn't installed.
	quitPause         = 200 * time.Millisecond

	installHint = "To show the crosshair, run: pip install PySide6. Then restart Freezy Aim."
	stoppedHint = "The crosshair stopped unexpectedly. Restart Freezy Aim to bring it back."
)

// Chromium-based browsers can show the app in its own plain window.
var appBrowsers = []string{
	"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser",
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
	"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
}

func opener() string {
	if runtime.GOOS == "darwin" {
		return "open"
	}
	return "xdg-open"
}

// openWindow returns the app window's process, if it has its own.
func openWindow(address string) *exec.Cmd {
	if args, err := appWindowArgs(address); err == nil {
		for _, browser := range appBrowsers {
			path, err := exec.LookPath(browser)
			if err != nil {
				continue
			}
			command := exec.Command(path, args...)
			if command.Start() == nil {
				return command
			}
		}
	}
	if err := exec.Command(opener(), address).Start(); err != nil {
		fmt.Println("Open this link in your browser:", address)
	}
	return nil
}

// unixOverlay hands the crosshair to overlay.py through files in the data folder.
type unixOverlay struct {
	folder  string
	visible atomic.Bool
	problem atomic.Value
	quit    chan struct{}
	closing sync.Once
}

func (o *unixOverlay) Quit() { o.closing.Do(func() { close(o.quit) }) }

func (o *unixOverlay) path(name string) string { return filepath.Join(o.folder, name) }

// publish draws the crosshair and tells overlay.py where it goes.
func (o *unixOverlay) publish() {
	config, err := LoadConfig(o.path(configName))
	if err != nil {
		log.Printf("reading settings: %v", err)
		return
	}
	state, err := json.Marshal(map[string]any{
		"visible":  o.visible.Load(),
		"opacity":  config.Opacity,
		"offset_x": config.OffsetX,
		"offset_y": config.OffsetY,
		"screen":   config.Screen,
	})
	if err != nil {
		log.Printf("saving the crosshair position: %v", err)
		return
	}
	// The pictures go first, so overlay.json never points at old ones.
	for name, scale := range map[string]int{overlayImageName: 1, overlaySharpName: 2} {
		var picture bytes.Buffer
		if err := writePNG(&picture, Render(config.Scaled(scale))); err != nil {
			log.Printf("drawing the crosshair: %v", err)
			return
		}
		if err := writeFileSafely(o.path(name), picture.Bytes()); err != nil {
			log.Printf("saving the crosshair: %v", err)
			return
		}
	}
	if err := writeFileSafely(o.path(overlayStateName), state); err != nil {
		log.Printf("saving the crosshair position: %v", err)
	}
}

func (o *unixOverlay) Changed()      { o.publish() }
func (o *unixOverlay) Visible() bool { return o.visible.Load() }

func (o *unixOverlay) SetVisible(on bool) {
	o.visible.Store(on)
	o.publish()
}

func (o *unixOverlay) Monitors() int {
	var status struct {
		Monitors int `json:"monitors"`
	}
	data, err := os.ReadFile(o.path(overlayStatusName))
	if err != nil || json.Unmarshal(data, &status) != nil || status.Monitors < 1 {
		return 1
	}
	return status.Monitors
}

func (o *unixOverlay) OpenFolder() {
	if err := exec.Command(opener(), o.folder).Start(); err != nil {
		log.Printf("opening the data folder: %v", err)
	}
}

func (o *unixOverlay) Problem() string {
	text, _ := o.problem.Load().(string)
	return text
}

// Global shortcuts and starting with the system are Windows only for now.
func (*unixOverlay) SystemFeatures() bool             { return false }
func (*unixOverlay) ApplyHotkeys(_, _ Hotkey) error   { return nil }
func (*unixOverlay) StartsWithWindows() bool          { return false }
func (*unixOverlay) SetStartWithWindows(_ bool) error { return nil }
func (*unixOverlay) Window(string, int)               {}

// runHelper shows the crosshair until the tray's Quit closes it. If it can't run, the
// app keeps going so crosshairs can still be edited, and the window explains why.
func (o *unixOverlay) runHelper(command *exec.Cmd, address string) {
	script := o.path(overlayScriptName)
	if err := os.WriteFile(script, overlayScript, 0o644); err != nil {
		log.Printf("saving the overlay helper: %v", err)
		o.problem.Store(stoppedHint)
		return
	}
	icon, err := iconPNG()
	if err == nil {
		err = os.WriteFile(o.path(trayIconName), icon, 0o644)
	}
	if err != nil {
		log.Printf("saving the tray icon: %v", err)
	}
	command.Args = append(command.Args, script, o.folder, address)
	command.Stderr = os.Stderr
	err = command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		o.Quit()
	case errors.Is(err, exec.ErrNotFound), errors.As(err, &exit) && exit.ExitCode() == missingQtExit:
		o.problem.Store(installHint)
		fmt.Fprintln(os.Stderr, installHint)
	default:
		log.Printf("overlay helper: %v", err)
		o.problem.Store(stoppedHint)
	}
}

// openRunning shows the window of a copy that's already running, if there is one.
func openRunning(folder string) bool {
	data, err := os.ReadFile(filepath.Join(folder, instanceName))
	if err != nil {
		return false
	}
	address := strings.TrimSpace(string(data))
	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}
	connection, err := net.DialTimeout("tcp", parsed.Host, 300*time.Millisecond)
	if err != nil {
		return false
	}
	connection.Close()
	openWindow(address)
	return true
}

// previewOverlay is for working on the app window: `go run . some-folder`. It does
// nothing on screen, and turns on every feature so all of Settings can be tested.
type previewOverlay struct {
	visible atomic.Bool
	startup atomic.Bool
}

func (*previewOverlay) Changed()                       {}
func (o *previewOverlay) Visible() bool                { return o.visible.Load() }
func (o *previewOverlay) SetVisible(on bool)           { o.visible.Store(on) }
func (*previewOverlay) Monitors() int                  { return 2 }
func (*previewOverlay) OpenFolder()                    {}
func (*previewOverlay) Quit()                          { fmt.Println("quit") }
func (*previewOverlay) Problem() string                { return "" }
func (*previewOverlay) Window(string, int)             {}
func (*previewOverlay) SystemFeatures() bool           { return true }
func (*previewOverlay) ApplyHotkeys(_, _ Hotkey) error { return nil }
func (o *previewOverlay) StartsWithWindows() bool      { return o.startup.Load() }
func (o *previewOverlay) SetStartWithWindows(on bool) error {
	o.startup.Store(on)
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func runPreview(folder string) {
	overlay := &previewOverlay{}
	overlay.visible.Store(true)
	store, err := OpenStore(folder, overlay)
	if err != nil {
		fail(err)
	}
	server, err := StartServer(store)
	if err != nil {
		fail(err)
	}
	fmt.Println(server.URL())
	select {}
}

func main() {
	if len(os.Args) > 1 {
		runPreview(os.Args[1])
	}
	executable, err := os.Executable()
	if err != nil {
		fail(err)
	}
	folder := DataFolder(filepath.Dir(executable))
	if openRunning(folder) {
		return
	}
	if logFile, err := StartLog(folder); err == nil {
		defer logFile.Close()
	}

	overlay := &unixOverlay{folder: folder, quit: make(chan struct{})}
	overlay.visible.Store(true)
	store, err := OpenStore(folder, overlay)
	if err != nil {
		fail(err)
	}
	server, err := StartServer(store)
	if err != nil {
		fail(err)
	}
	address := server.URL()
	instance := filepath.Join(folder, instanceName)
	if err := os.WriteFile(instance, []byte(address), 0o600); err != nil {
		log.Printf("saving the app address: %v", err)
	}
	defer os.Remove(instance)

	overlay.publish()
	fmt.Println("Freezy Aim is running. Close this terminal or press Ctrl+C to quit.")
	var window *exec.Cmd
	if !store.Options().StartInTray {
		window = openWindow(address)
	}

	helper := exec.Command("python3")
	go overlay.runHelper(helper, address)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-overlay.quit:
	case <-stop:
	}
	running := []*exec.Cmd{helper, window}
	for _, process := range running {
		if process != nil && process.Process != nil {
			process.Process.Signal(syscall.SIGTERM)
		}
	}
	time.Sleep(quitPause) // Lets the close button's reply reach the window.
	// Anything still running is stopped outright, so the crosshair can't stay on screen.
	for _, process := range running {
		if process != nil && process.Process != nil {
			process.Process.Kill()
		}
	}
}
