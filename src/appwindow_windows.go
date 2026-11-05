//go:build windows

// Opens the app window, which shows the local page from server.go. It's our own window
// using WebView2, or an Edge "app mode" window if that can't be used. It can be closed
// while gaming so only the small overlay keeps running.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	launchPause      = 3 * time.Second // Stops a double click opening two windows.
	closeWait        = 4 * time.Second // How long another version gets to close.
	quitTimeout      = 2 * time.Second // How long the red X waits before forcing the app to end.
	maxPathLen       = 1024
	queryLimitedInfo = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	browserWait      = 300 * time.Millisecond

	// Versions before the rename to Freezy Aim, which didn't check for each other.
	oldClassName = "CrosshairOverlay"
	oldHostTitle = "Crosshair Overlay host"

	browserWindowClass = "Chrome_WidgetWin_1"
)

var (
	enumWindows        = user32.NewProc("EnumWindows")
	findWindow         = user32.NewProc("FindWindowW")
	findWindowEx       = user32.NewProc("FindWindowExW")
	getWindowText      = user32.NewProc("GetWindowTextW")
	isWindow           = user32.NewProc("IsWindow")
	getWindowProcessID = user32.NewProc("GetWindowThreadProcessId")
	getWindowTextLen   = user32.NewProc("GetWindowTextLengthW")
	isIconic           = user32.NewProc("IsIconic")
	isWindowVisible    = user32.NewProc("IsWindowVisible")

	queryImageName = kernel32.NewProc("QueryFullProcessImageNameW")

	windowSearch = syscall.NewCallback(checkWindow)

	// previousFolder is where a replaced version ran from, so its settings can come along.
	previousFolder string

	appURL     string
	appProcess atomic.Pointer[os.Process]
	lastLaunch time.Time
	searchPID  uint32
	foundApp   uintptr
)

// windowsOverlay lets the server's threads reach the overlay. They only post
// messages, so all window work still happens on the main thread.
type windowsOverlay struct{}

func (windowsOverlay) Changed()      { postMessage.Call(host, wmSettings, 0, 0) }
func (windowsOverlay) Visible() bool { return visible.Load() }
func (windowsOverlay) Monitors() int { return int(monitorCount.Load()) }
func (windowsOverlay) OpenFolder()   { postMessage.Call(host, wmOpenFolder, 0, 0) }

func (windowsOverlay) SystemFeatures() bool { return true }
func (windowsOverlay) Problem() string      { return "" }

func (windowsOverlay) SetVisible(on bool) {
	var value uintptr
	if on {
		value = 1
	}
	postMessage.Call(host, wmShowOverlay, value, 0)
}

// runningHost finds the hidden window of a copy that's already running.
func runningHost() (window uintptr, sameVersion bool) {
	var after uintptr
	for {
		found, _, _ := findWindowEx.Call(0, after, uintptr(unsafe.Pointer(wide(windowClassName))), 0)
		if found == 0 {
			return 0, false
		}
		title := make([]uint16, 64)
		length, _, _ := getWindowText.Call(found, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		name := syscall.UTF16ToString(title[:length])
		if strings.HasPrefix(name, hostPrefix) {
			return found, name == hostPrefix+" "+version
		}
		after = found
	}
}

// windowFolder returns the folder of the program that owns a window.
func windowFolder(window uintptr) string {
	var pid uint32
	getWindowProcessID.Call(window, uintptr(unsafe.Pointer(&pid)))
	process, err := syscall.OpenProcess(queryLimitedInfo, false, pid)
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(process)
	path := make([]uint16, maxPathLen)
	size := uint32(len(path))
	ok, _, _ := queryImageName.Call(uintptr(process), 0, uintptr(unsafe.Pointer(&path[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return ""
	}
	return filepath.Dir(syscall.UTF16ToString(path[:size]))
}

// closeAndWait asks a window to close, and reports whether it did.
func closeAndWait(window uintptr) bool {
	postMessage.Call(window, wmClose, 0, 0)
	for deadline := time.Now().Add(closeWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if open, _, _ := isWindow.Call(window); open == 0 {
			return true
		}
	}
	return false
}

// takeOver decides what happens when a copy is already running. The same version
// just shows its window. Any other version closes, so opening a new download always
// runs that download. It returns true if this copy should carry on starting.
func takeOver() bool {
	running, sameVersion := runningHost()
	if sameVersion {
		postMessage.Call(running, wmOpenApp, 0, 0)
		return false
	}
	if running != 0 {
		previousFolder = windowFolder(running)
		if !closeAndWait(running) {
			alert("Please quit Freezy Aim from its tray icon, then open this version again.")
			return false
		}
	}
	// The old version's app window would otherwise stay open with nothing behind it.
	stale, _, _ := findWindow.Call(uintptr(unsafe.Pointer(wide(browserWindowClass))), uintptr(unsafe.Pointer(wide(appName))))
	if stale != 0 {
		closeAndWait(stale)
		time.Sleep(browserWait)
	}
	return true
}

// closeLegacyCopy closes a copy from before the rename to Freezy Aim. Those don't share
// the check for a running copy, and would leave a second crosshair on screen.
func closeLegacyCopy() {
	old, _, _ := findWindow.Call(uintptr(unsafe.Pointer(wide(oldClassName))), uintptr(unsafe.Pointer(wide(oldHostTitle))))
	if old != 0 {
		closeAndWait(old)
	}
}

// browserPath finds Edge, or Chrome if Edge has been removed.
func browserPath() string {
	roots := []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")}
	for _, program := range []string{`Microsoft\Edge\Application\msedge.exe`, `Google\Chrome\Application\chrome.exe`} {
		for _, root := range roots {
			path := filepath.Join(root, program)
			if root == "" {
				continue
			}
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return ""
}

func checkWindow(window, _ uintptr) uintptr {
	var pid uint32
	getWindowProcessID.Call(window, uintptr(unsafe.Pointer(&pid)))
	if pid != searchPID {
		return 1
	}
	showing, _, _ := isWindowVisible.Call(window)
	titled, _, _ := getWindowTextLen.Call(window)
	if showing == 0 || titled == 0 {
		return 1
	}
	foundApp = window
	return 0 // Found it, so stop searching.
}

// Quit closes everything through the main thread, which removes the crosshair and tray
// icon. If that's stuck for any reason, the process ends anyway, which also removes them.
func (windowsOverlay) Quit() {
	postMessage.Call(host, wmQuitApp, 0, 0)
	time.AfterFunc(quitTimeout, func() { os.Exit(0) })
}

// findApp looks for the open app window.
func findApp(process *os.Process) uintptr {
	searchPID, foundApp = uint32(process.Pid), 0
	enumWindows.Call(windowSearch, 0)
	return foundApp
}

// focusApp brings an open app window to the front.
func focusApp(process *os.Process) bool {
	if findApp(process) == 0 {
		return false
	}
	focusWindow(foundApp)
	return true
}

func focusWindow(window uintptr) {
	if minimized, _, _ := isIconic.Call(window); minimized != 0 {
		showWindow.Call(window, swRestore)
	}
	setForegroundWindow.Call(window)
}

func openInDefaultBrowser() {
	shellExecute.Call(0, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(appURL))), 0, 0, swShowNormal)
}

// openApp shows the app window, reusing one that's already open.
func openApp() {
	if appURL == "" || time.Since(lastLaunch) < launchPause {
		return
	}
	if window := pageHost.Load(); window != 0 {
		focusWindow(window)
		return
	}
	if process := appProcess.Load(); process != nil && focusApp(process) {
		return
	}
	lastLaunch = time.Now()
	if openHostedApp() {
		return
	}

	browser := browserPath()
	args, err := appWindowArgs(appURL)
	if browser == "" || err != nil {
		openInDefaultBrowser()
		return
	}
	command := exec.Command(browser, args...)
	if err := command.Start(); err != nil {
		openInDefaultBrowser()
		return
	}
	appProcess.Store(command.Process)
	go func() {
		command.Wait()
		appProcess.CompareAndSwap(command.Process, nil)
	}()
}

// closeApp asks the window to close, so the next launch starts cleanly.
func closeApp() {
	if window := pageHost.Load(); window != 0 {
		postMessage.Call(window, wmClose, 0, 0)
		return
	}
	process := appProcess.Load()
	if process == nil {
		return
	}
	if window := findApp(process); window != 0 {
		postMessage.Call(window, wmClose, 0, 0)
		return
	}
	process.Kill()
}
