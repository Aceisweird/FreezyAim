//go:build windows

// The Windows overlay: a small, click-through window that stays on top, plus a
// tray icon. It uses only Go's standard library, so the .exe has no dependencies.
package main

import (
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

const (
	pollMs              = 1000 // How often to check the settings file and stacking order.
	settleWait          = 15 * time.Second
	gwHwndPrev          = 3
	belowNormalPriority = 0x4000
	currentProcess      = ^uintptr(0) // The handle that always means this process.
	windowClassName     = "FreezyAim"
	hostPrefix          = "Freezy Aim host" // Followed by the version, so copies can tell each other apart.

	wsPopup          = 0x80000000
	wsExTopmost      = 0x00000008
	wsExTransparent  = 0x00000020 // With wsExLayered, clicks pass through.
	wsExToolWindow   = 0x00000080 // Keeps it out of the taskbar and Alt+Tab.
	wsExLayered      = 0x00080000
	wsExNoActivate   = 0x08000000 // It never takes focus from the game.
	swHide           = 0
	swShowNormal     = 1
	swShowNoActivate = 4
	swRestore        = 9
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoZOrder      = 0x0004
	swpNoActivate    = 0x0010
	hwndTopmost      = ^uintptr(0) // HWND_TOPMOST is -1.
	dpiPerMonitorV2  = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is -4.
	wmNull           = 0x0000
	wmClose          = 0x0010
	wmDisplayChange  = 0x007E
	wmHotkey         = 0x0312
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmTimer          = 0x0113
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmTray           = 0x8001 // WM_APP + 1: our tray icon's clicks.
	wmSettings       = 0x8002 // The app window rewrote the settings file.
	wmShowOverlay    = 0x8003 // The app window showed or hid the crosshair.
	wmOpenApp        = 0x8004 // A second copy of the program was started.
	wmOpenFolder     = 0x8005 // The app window asked to show the data folder.
	wmSetHotkeys     = 0x8006 // Settings changed the shortcuts.
	wmQuitApp        = 0x8007 // The red X in the app window.
	nimAdd           = 0
	nimModify        = 1
	nimDelete        = 2
	nifMessage       = 0x1
	nifIcon          = 0x2
	nifTip           = 0x4
	mfString         = 0x000
	mfChecked        = 0x008
	mfSeparator      = 0x800
	tpmRightButton   = 0x0002
	tpmBottomAlign   = 0x0020
	ulwAlpha         = 2
	acSrcAlpha       = 1
	monitorPrimary   = 1
	mbIconError      = 0x10

	menuToggle = 1
	menuOpen   = 2
	menuQuit   = 3
	menuEdge   = 4
)

type point struct{ X, Y int32 }

type winRect struct{ Left, Top, Right, Bottom int32 }

type message struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

type windowClass struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type bitmapHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type blendFunction struct {
	BlendOp     byte
	BlendFlags  byte
	Alpha       byte
	AlphaFormat byte
}

type monitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

type iconInfo struct {
	IsIcon   int32
	HotspotX uint32
	HotspotY uint32
	Mask     uintptr
	Color    uintptr
}

type notifyIconData struct {
	Size            uint32
	Hwnd            uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

// These stop the build if a struct doesn't match Windows' 64-bit layout.
var (
	_ [0]struct{} = [unsafe.Sizeof(message{}) - 48]struct{}{}
	_ [0]struct{} = [unsafe.Sizeof(windowClass{}) - 80]struct{}{}
	_ [0]struct{} = [unsafe.Sizeof(bitmapHeader{}) - 40]struct{}{}
	_ [0]struct{} = [unsafe.Sizeof(monitorInfo{}) - 40]struct{}{}
	_ [0]struct{} = [unsafe.Sizeof(iconInfo{}) - 32]struct{}{}
	_ [0]struct{} = [unsafe.Sizeof(notifyIconData{}) - 976]struct{}{}
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	appendMenu            = user32.NewProc("AppendMenuW")
	createIconIndirect    = user32.NewProc("CreateIconIndirect")
	createPopupMenu       = user32.NewProc("CreatePopupMenu")
	createWindowEx        = user32.NewProc("CreateWindowExW")
	defWindowProc         = user32.NewProc("DefWindowProcW")
	destroyMenu           = user32.NewProc("DestroyMenu")
	destroyWindow         = user32.NewProc("DestroyWindow")
	dispatchMessage       = user32.NewProc("DispatchMessageW")
	enumDisplayMonitors   = user32.NewProc("EnumDisplayMonitors")
	getCursorPos          = user32.NewProc("GetCursorPos")
	getDC                 = user32.NewProc("GetDC")
	getMessage            = user32.NewProc("GetMessageW")
	getMonitorInfo        = user32.NewProc("GetMonitorInfoW")
	messageBox            = user32.NewProc("MessageBoxW")
	postMessage           = user32.NewProc("PostMessageW")
	postQuitMessage       = user32.NewProc("PostQuitMessage")
	registerClassEx       = user32.NewProc("RegisterClassExW")
	registerWindowMessage = user32.NewProc("RegisterWindowMessageW")
	releaseDC             = user32.NewProc("ReleaseDC")
	setDpiAwareness       = user32.NewProc("SetProcessDpiAwarenessContext")
	setForegroundWindow   = user32.NewProc("SetForegroundWindow")
	setTimer              = user32.NewProc("SetTimer")
	getWindow             = user32.NewProc("GetWindow")
	setPriorityClass      = kernel32.NewProc("SetPriorityClass")
	setWindowPos          = user32.NewProc("SetWindowPos")
	showWindow            = user32.NewProc("ShowWindow")
	trackPopupMenu        = user32.NewProc("TrackPopupMenu")
	translateMessage      = user32.NewProc("TranslateMessage")
	updateLayeredWindow   = user32.NewProc("UpdateLayeredWindow")

	createBitmap       = gdi32.NewProc("CreateBitmap")
	createCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	createDIBSection   = gdi32.NewProc("CreateDIBSection")
	deleteDC           = gdi32.NewProc("DeleteDC")
	deleteObject       = gdi32.NewProc("DeleteObject")
	selectObject       = gdi32.NewProc("SelectObject")

	shellExecute    = shell32.NewProc("ShellExecuteW")
	shellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

	createMutex     = kernel32.NewProc("CreateMutexW")
	getModuleHandle = kernel32.NewProc("GetModuleHandleW")
)

// Windows callbacks can't be freed, so each one is created once. Making one on
// every poll would crash the app after a few minutes.
var monitorCallback = syscall.NewCallback(enumMonitor)

// Everything here runs on the one locked thread, so it needs no locks.
var (
	overlay        uintptr // The visible crosshair window.
	host           uintptr // A hidden window that owns the tray icon and menu.
	trayIcon       uintptr
	taskbarCreated uint32 // Sent when Explorer restarts and the tray needs refilling.

	settings    = DefaultConfig()
	settingsErr string
	configPath  string
	dataPath    string
	configTime  time.Time

	picture  Image
	redraw   = true
	windowX  int32
	windowY  int32
	monitors []winRect

	// The server's threads read these too.
	visible      atomic.Bool
	monitorCount atomic.Int32
)

// Windows delivers a window's messages to the thread that created it.
func init() { runtime.LockOSThread() }

func wide(text string) *uint16 {
	pointer, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		pointer, _ = syscall.UTF16PtrFromString("?")
	}
	return pointer
}

func alert(text string) {
	messageBox.Call(0, uintptr(unsafe.Pointer(wide(text))), uintptr(unsafe.Pointer(wide(appName))), mbIconError)
}

func firstInstance() bool {
	handle, _, err := createMutex.Call(0, 0, uintptr(unsafe.Pointer(wide("Local\\FreezyAim"))))
	return handle == 0 || err != syscall.ERROR_ALREADY_EXISTS
}

// loadSettings applies the settings file. A bad file keeps the last good
// settings, and the tray tooltip says why.
func loadSettings() {
	loaded, err := LoadConfig(configPath)
	if err != nil {
		settingsErr = err.Error()
	} else {
		settings, settingsErr, redraw = loaded, "", true
	}
	if info, err := os.Stat(configPath); err == nil {
		configTime = info.ModTime()
	}
}

func settingsChanged() bool {
	info, err := os.Stat(configPath)
	return err == nil && !info.ModTime().Equal(configTime)
}

// newBitmap copies an image into a bitmap that Windows can use.
func newBitmap(dc uintptr, img Image) uintptr {
	header := bitmapHeader{
		Width:    int32(img.Side),
		Height:   -int32(img.Side), // Negative means rows run top to bottom.
		Planes:   1,
		BitCount: 32,
	}
	header.Size = uint32(unsafe.Sizeof(header))
	var bits unsafe.Pointer
	bitmap, _, _ := createDIBSection.Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap != 0 {
		copy(unsafe.Slice((*byte)(bits), len(img.Pix)), img.Pix)
	}
	return bitmap
}

func createTrayIcon() uintptr {
	img, err := appIcon()
	if err != nil {
		log.Printf("tray icon: %v", err)
		img = Render(DefaultConfig())
	}

	screen, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, screen)
	color := newBitmap(screen, img)
	defer deleteObject.Call(color)
	mask, _, _ := createBitmap.Call(uintptr(img.Side), uintptr(img.Side), 1, 1, 0)
	defer deleteObject.Call(mask)

	info := iconInfo{IsIcon: 1, Mask: mask, Color: color}
	icon, _, _ := createIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}

func updateTray(action uintptr) {
	data := notifyIconData{
		Hwnd:            host,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: wmTray,
		Icon:            trayIcon,
	}
	data.Size = uint32(unsafe.Sizeof(data))
	tip := appName
	if settingsErr != "" {
		tip = appName + ": settings not applied: " + settingsErr
	}
	copy(data.Tip[:len(data.Tip)-1], utf16.Encode([]rune(tip)))
	shellNotifyIcon.Call(action, uintptr(unsafe.Pointer(&data)))
}

func showMenu() {
	menu, _, _ := createPopupMenu.Call()
	defer destroyMenu.Call(menu)
	toggleFlags := uintptr(mfString)
	if visible.Load() {
		toggleFlags |= mfChecked
	}
	appendMenu.Call(menu, mfString, menuOpen, uintptr(unsafe.Pointer(wide("Open "+appName))))
	if pageHost.Load() != 0 {
		appendMenu.Call(menu, mfString, menuEdge, uintptr(unsafe.Pointer(wide("Open in Edge instead"))))
	}
	appendMenu.Call(menu, toggleFlags, menuToggle, uintptr(unsafe.Pointer(wide("Show crosshair"))))
	appendMenu.Call(menu, mfSeparator, 0, 0)
	appendMenu.Call(menu, mfString, menuQuit, uintptr(unsafe.Pointer(wide("Quit"))))

	var cursor point
	getCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	// Without these two calls around it, the menu won't close on an outside click.
	setForegroundWindow.Call(host)
	trackPopupMenu.Call(menu, tpmRightButton|tpmBottomAlign, uintptr(cursor.X), uintptr(cursor.Y), 0, host, 0)
	postMessage.Call(host, wmNull, 0, 0)
}

func enumMonitor(handle, _, _, _ uintptr) uintptr {
	info := monitorInfo{}
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, _ := getMonitorInfo.Call(handle, uintptr(unsafe.Pointer(&info)))
	if ok != 0 {
		if info.Flags&monitorPrimary != 0 {
			monitors = append([]winRect{info.Monitor}, monitors...)
		} else {
			monitors = append(monitors, info.Monitor)
		}
	}
	return 1
}

// present puts the current picture on screen at (x, y).
func present(x, y int32) {
	screen, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, screen)
	memory, _, _ := createCompatibleDC.Call(screen)
	defer deleteDC.Call(memory)
	bitmap := newBitmap(screen, picture)
	if bitmap == 0 {
		return
	}
	defer deleteObject.Call(bitmap)
	previous, _, _ := selectObject.Call(memory, bitmap)
	defer selectObject.Call(memory, previous)

	position := point{x, y}
	size := point{int32(picture.Side), int32(picture.Side)}
	source := point{}
	blend := blendFunction{
		Alpha:       byte(math.Round(settings.Opacity * 255)),
		AlphaFormat: acSrcAlpha,
	}
	updateLayeredWindow.Call(
		overlay, screen,
		uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&size)),
		memory, uintptr(unsafe.Pointer(&source)),
		0, uintptr(unsafe.Pointer(&blend)), ulwAlpha,
	)
}

// place centres the crosshair on the chosen monitor, redrawing only if needed.
func place() {
	monitors = monitors[:0]
	enumDisplayMonitors.Call(0, 0, monitorCallback, 0)
	monitorCount.Store(int32(len(monitors)))
	if len(monitors) == 0 {
		return
	}
	area := monitors[0]
	if settings.Screen >= 0 && settings.Screen < len(monitors) {
		area = monitors[settings.Screen]
	}
	if redraw {
		picture = Render(settings).Premultiplied()
	}
	side := int32(picture.Side)
	x := area.Left + (area.Right-area.Left-side)/2 + int32(settings.OffsetX)
	y := area.Top + (area.Bottom-area.Top-side)/2 + int32(settings.OffsetY)
	if redraw || x != windowX || y != windowY {
		present(x, y)
		windowX, windowY, redraw = x, y, false
	}
}

func poll() {
	if settingsChanged() {
		loadSettings()
		updateTray(nimModify)
		place()
	}
	if visible.Load() && coveredByWindow() {
		// Some games push topmost windows down, so this reclaims the top spot.
		setWindowPos.Call(overlay, hwndTopmost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
	}
}

// coveredByWindow checks for a visible window above the crosshair. It's only a read, so
// the stacking order isn't touched while a game runs unless something has really changed.
func coveredByWindow() bool {
	above := overlay
	for {
		above, _, _ = getWindow.Call(above, gwHwndPrev)
		if above == 0 {
			return false
		}
		if showing, _, _ := isWindowVisible.Call(above); showing != 0 {
			return true
		}
	}
}

// yieldToGames lowers this process so its occasional work can't take time from a game.
func yieldToGames() { setPriorityClass.Call(currentProcess, belowNormalPriority) }

// quitApp closes everything: the crosshair, the tray icon and the app window.
func quitApp() {
	destroyWindow.Call(overlay)
	destroyWindow.Call(host)
}

func setVisible(on bool) {
	visible.Store(on)
	if on {
		showWindow.Call(overlay, swShowNoActivate)
	} else {
		showWindow.Call(overlay, swHide)
	}
}

func windowProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch uint32(msg) {
	case wmTimer:
		poll()
		return 0
	case wmTray:
		switch uint32(lParam) {
		case wmLButtonUp:
			openApp()
		case wmRButtonUp:
			showMenu()
		}
		return 0
	case wmSettings:
		poll()
		return 0
	case wmShowOverlay:
		setVisible(wParam != 0)
		return 0
	case wmOpenApp:
		openApp()
		return 0
	case wmWebFallback:
		useBrowserWindow()
		return 0
	case wmQuitApp:
		quitApp()
		return 0
	case wmDisplayChange:
		place()
		return 0
	case wmHotkey:
		switch wParam {
		case hotkeyToggle:
			setVisible(!visible.Load())
		case hotkeyNext:
			go appStore.NextFavorite() // It saves files, so it stays off this thread.
		}
		return 0
	case wmSetHotkeys:
		keys := pendingKeys.Load()
		if applyHotkeys(keys[0], keys[1]) {
			return 0
		}
		return 1
	case wmOpenFolder:
		shellExecute.Call(0, uintptr(unsafe.Pointer(wide("open"))), uintptr(unsafe.Pointer(wide(dataPath))), 0, 0, swShowNormal)
		return 0
	case wmCommand:
		switch wParam & 0xffff {
		case menuOpen:
			openApp()
		case menuEdge:
			useBrowserWindow()
		case menuToggle:
			setVisible(!visible.Load())
		case menuQuit:
			quitApp()
		}
		return 0
	case wmDestroy:
		if hwnd == host {
			closeApp()
			updateTray(nimDelete)
			postQuitMessage.Call(0)
		}
		return 0
	case taskbarCreated:
		updateTray(nimAdd)
		return 0
	}
	result, _, _ := defWindowProc.Call(hwnd, msg, wParam, lParam)
	return result
}

func createWindows() bool {
	instance, _, _ := getModuleHandle.Call(0)
	className := wide(windowClassName)
	class := windowClass{
		WndProc:   syscall.NewCallback(windowProc),
		Instance:  instance,
		ClassName: className,
	}
	class.Size = uint32(unsafe.Sizeof(class))
	if atom, _, _ := registerClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return false
	}
	create := func(title string, exStyle uintptr) uintptr {
		window, _, _ := createWindowEx.Call(exStyle, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wide(title))),
			wsPopup, 0, 0, 1, 1, 0, 0, instance, 0)
		return window
	}
	host = create(hostPrefix+" "+version, wsExToolWindow)
	overlay = create(appName, wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate)
	return host != 0 && overlay != 0
}

func main() {
	defer func() {
		if problem := recover(); problem != nil {
			log.Printf("crash: %v\n%s", problem, debug.Stack())
			alert("Freezy Aim had a problem and needs to close. Details are in " + filepath.Join(dataPath, logName))
			os.Exit(1)
		}
	}()

	// Sizes are real pixels, and display scaling can't blur or shift the crosshair.
	if setDpiAwareness.Find() == nil {
		setDpiAwareness.Call(dpiPerMonitorV2)
	}
	if !firstInstance() && !takeOver() {
		return
	}
	closeLegacyCopy()
	executable, err := os.Executable()
	if err != nil {
		alert("Couldn't find the settings folder: " + err.Error())
		return
	}
	appFolder := filepath.Dir(executable)
	dataPath = DataFolder(appFolder)
	if logFile, err := StartLog(dataPath); err == nil {
		defer logFile.Close()
	}
	for _, folder := range []string{previousFolder, appFolder} {
		if folder == "" {
			continue
		}
		if err := CopyOldData(folder, dataPath); err != nil {
			log.Printf("copying old settings: %v", err)
		}
	}
	configPath = filepath.Join(dataPath, configName)
	visible.Store(true)

	registered, _, _ := registerWindowMessage.Call(uintptr(unsafe.Pointer(wide("TaskbarCreated"))))
	taskbarCreated = uint32(registered)
	if !createWindows() {
		alert("Couldn't create the crosshair window.")
		return
	}
	trayIcon = createTrayIcon()
	store, storeErr := OpenStore(dataPath, windowsOverlay{})
	loadSettings()
	updateTray(nimAdd)
	place()
	showWindow.Call(overlay, swShowNoActivate)
	setTimer.Call(host, 1, pollMs, 0)
	time.AfterFunc(settleWait, yieldToGames) // Full speed until the app window has opened.

	// The crosshair still works if the app window can't start.
	if storeErr != nil {
		log.Printf("opening saved crosshairs: %v", storeErr)
		alert("Couldn't open your saved crosshairs: " + storeErr.Error())
	} else if server, err := StartServer(store); err != nil {
		log.Printf("starting the app window: %v", err)
		alert("Couldn't start the app window: " + err.Error())
	} else {
		appStore = store
		appURL = server.URL()
		options := store.Options()
		if !applyHotkeys(options.ToggleKey, options.NextKey) {
			log.Printf("a saved shortcut is in use by another app")
		}
		// Keeps the startup entry right if the app has been moved.
		if startup := (windowsOverlay{}); startup.StartsWithWindows() {
			if err := startup.SetStartWithWindows(true); err != nil {
				log.Printf("updating the startup entry: %v", err)
			}
		}
		if !options.StartInTray {
			openApp()
		}
	}

	var msg message
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) <= 0 {
			return
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
