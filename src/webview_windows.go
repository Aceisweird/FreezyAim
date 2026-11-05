//go:build windows

// Hosts the app page in a window of our own using WebView2, the Edge component built
// into Windows 11, so there's no browser title bar to deal with. If that isn't available
// or the page doesn't load, the app falls back to a normal Edge window.
package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	pageWindowClass    = "FreezyAimApp"
	webRuntimeKey      = `Software\Microsoft\EdgeUpdate\ClientState\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	webRuntimeValue    = "EBWebView"
	webRuntimeFolder   = `Microsoft\EdgeWebView\Application\*`
	webEntryPoint      = "CreateWebViewEnvironmentWithOptionsInternal"
	webRegistry32      = 0x0200 // KEY_WOW64_32KEY
	webWait            = 12 * time.Second
	webBackground      = "FF000000"
	webArguments       = "--disable-background-networking --disable-features=msSmartScreenProtection"
	apartmentThreaded  = 2
	pageWidth          = 1240
	pageHeight         = 820
	baseDPI            = 96
	wsOverlappedWindow = 0x00CF0000
	useDefaultPosition = 0x80000000
	blackBrush         = 4
	smCxFrame          = 32
	smCyFrame          = 33
	smCxPaddedBorder   = 92
	swpFrameChanged    = 0x0020
	sizeMinimized      = 1
	wmMove             = 0x0003
	wmSize             = 0x0005
	wmSetFocus         = 0x0007
	wmNcCalcSize       = 0x0083
	wmDpiChanged       = 0x02E0
	wmWebFallback      = 0x8008 // The app page couldn't be hosted.

	// Positions in the WebView2 COM interfaces, which Microsoft keeps stable.
	comQuery                  = 0
	comAddRef                 = 1
	comRelease                = 2
	environmentNewController  = 3
	controllerPutVisible      = 4
	controllerPutBounds       = 6
	controllerMoveFocus       = 12
	controllerParentMoved     = 23
	controllerClose           = 24
	controllerPage            = 25
	pageSettings              = 3
	pageNavigate              = 5
	settingsPutStatusBar      = 10
	moveFocusProgrammatically = 0
)

var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	coInitialize       = ole32.NewProc("CoInitializeEx")
	getStockObject     = gdi32.NewProc("GetStockObject")
	getClientRect      = user32.NewProc("GetClientRect")
	getDpiForSystem    = user32.NewProc("GetDpiForSystem")
	getDpiForWindow    = user32.NewProc("GetDpiForWindow")
	getSystemMetricDPI = user32.NewProc("GetSystemMetricsForDpi")
	loadIcon           = user32.NewProc("LoadIconW")

	// IID_ICoreWebView2Environment
	environmentID = [16]byte{0x5e, 0x75, 0x6d, 0xb9, 0x19, 0x03, 0x92, 0x4e, 0xa2, 0x96, 0x23, 0x43, 0x6f, 0x46, 0xa1, 0xfc}

	handlerSlots       = [3]uintptr{syscall.NewCallback(handlerQuery), syscall.NewCallback(handlerCount), syscall.NewCallback(handlerCount)}
	environmentHandler *comHandler
	controllerHandler  *comHandler

	pageClassRegistered bool           // Main thread only.
	pageHost            atomic.Uintptr // The window hosting the page, while it's open.
	framelessApp        atomic.Bool
	webLoaded           atomic.Bool
	webLaunch           atomic.Int32
	webUnavailable      bool   // Main thread only.
	pageShowing         = true // Main thread only.
	webEnvironment      *comObject
	webController       *comObject
	webPage             *comObject
)

// comObject is any COM object made by WebView2.
type comObject struct{ vtable *[32]uintptr }

// call runs a method of the object.
//
//go:uintptrescapes
func (o *comObject) call(slot int, args ...uintptr) uint32 {
	result, _, _ := syscall.SyscallN(o.vtable[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return uint32(result)
}

// comHandler is a COM object of ours, which WebView2 calls when a step has finished.
type comHandler struct{ vtable *[4]uintptr }

func newHandler(done func(*comHandler, uintptr, *comObject) uintptr) *comHandler {
	return &comHandler{&[4]uintptr{handlerSlots[0], handlerSlots[1], handlerSlots[2], syscall.NewCallback(done)}}
}

func handlerQuery(this *comHandler, _ uintptr, out **comHandler) uintptr {
	*out = this
	return 0
}

// The handlers live as long as the program, so there's nothing to count.
func handlerCount(*comHandler) uintptr { return 1 }

// Set here because the handlers refer back to the code that uses these two.
func init() {
	environmentHandler = newHandler(webEnvironmentReady)
	controllerHandler = newHandler(webControllerReady)
}

// webRuntimeLibrary finds the WebView2 runtime's DLL.
func webRuntimeLibrary() string {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	library := filepath.Join("EBWebView", arch, "EmbeddedBrowserWebView.dll")
	for _, root := range []syscall.Handle{syscall.HKEY_LOCAL_MACHINE, syscall.HKEY_CURRENT_USER} {
		var key syscall.Handle
		if syscall.RegOpenKeyEx(root, wide(webRuntimeKey), 0, syscall.KEY_QUERY_VALUE|webRegistry32, &key) != nil {
			continue
		}
		folder := make([]uint16, maxPathLen)
		size := uint32(len(folder) * 2)
		err := syscall.RegQueryValueEx(key, wide(webRuntimeValue), nil, nil, (*byte)(unsafe.Pointer(&folder[0])), &size)
		syscall.RegCloseKey(key)
		if err == nil {
			path := filepath.Join(syscall.UTF16ToString(folder), library)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	found, _ := filepath.Glob(filepath.Join(os.Getenv("ProgramFiles(x86)"), webRuntimeFolder, library))
	if len(found) > 0 {
		return found[len(found)-1]
	}
	return ""
}

// webAttempt is a file that only exists while the page is first loading. If it's still
// there at the next launch, WebView2 took the app down, so that version doesn't use it.
func webAttempt() string { return filepath.Join(dataPath, "webview-"+version+".try") }

// openHostedApp starts opening the page in our own window. WebView2 joins it once ready.
// It returns false if that can't even begin.
func openHostedApp() bool {
	if webUnavailable {
		return false
	}
	if _, err := os.Stat(webAttempt()); err == nil {
		log.Printf("app window: WebView2 didn't finish loading last time, so using Edge")
		webUnavailable = true
		return false
	}
	path := webRuntimeLibrary()
	if path == "" {
		log.Printf("app window: WebView2 isn't installed, so using Edge")
		return false
	}
	library := syscall.NewLazyDLL(path)
	create := library.NewProc(webEntryPoint)
	if err := create.Find(); err != nil {
		log.Printf("app window: couldn't load WebView2: %v", err)
		return false
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		log.Printf("app window: %v", err)
		return false
	}
	if !createPageWindow() {
		return false
	}
	os.WriteFile(webAttempt(), nil, 0o644)
	webLoaded.Store(false)
	launch := webLaunch.Add(1)
	time.AfterFunc(webWait, func() {
		if webLaunch.Load() == launch && pageHost.Load() != 0 && !webLoaded.Load() {
			log.Printf("app window: the page didn't load in WebView2")
			postMessage.Call(host, wmWebFallback, 0, 0)
		}
	})
	os.Setenv("WEBVIEW2_DEFAULT_BACKGROUND_COLOR", webBackground)
	os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", webArguments)
	coInitialize.Call(0, apartmentThreaded)
	folder := wide(filepath.Join(cacheDir, "FreezyAim", "webview"))
	result, _, _ := create.Call(1, 0, uintptr(unsafe.Pointer(folder)), 0, uintptr(unsafe.Pointer(environmentHandler)))
	if uint32(result) != 0 {
		log.Printf("app window: WebView2 refused to start: %#x", uint32(result))
		destroyWindow.Call(pageHost.Load())
		return false
	}
	return true
}

func webEnvironmentReady(_ *comHandler, result uintptr, created *comObject) uintptr {
	window := pageHost.Load()
	if window == 0 {
		return 0
	}
	if uint32(result) != 0 || created == nil {
		return webStepFailed("start", uint32(result))
	}
	if failed := created.call(comQuery, uintptr(unsafe.Pointer(&environmentID)), uintptr(unsafe.Pointer(&webEnvironment))); failed != 0 {
		return webStepFailed("read the environment", failed)
	}
	if failed := webEnvironment.call(environmentNewController, window, uintptr(unsafe.Pointer(controllerHandler))); failed != 0 {
		return webStepFailed("attach to the window", failed)
	}
	return 0
}

func webControllerReady(_ *comHandler, result uintptr, created *comObject) uintptr {
	window := pageHost.Load()
	if window == 0 {
		return 0
	}
	if uint32(result) != 0 || created == nil {
		return webStepFailed("create the view", uint32(result))
	}
	created.call(comAddRef)
	webController = created
	pageShowing = true
	if failed := webController.call(controllerPage, uintptr(unsafe.Pointer(&webPage))); failed != 0 || webPage == nil {
		return webStepFailed("reach the page", failed)
	}
	var settings *comObject
	if webPage.call(pageSettings, uintptr(unsafe.Pointer(&settings))) == 0 && settings != nil {
		settings.call(settingsPutStatusBar, 0)
		settings.call(comRelease)
	}
	fitPage(window)
	webController.call(controllerPutVisible, 1)
	webController.call(controllerMoveFocus, moveFocusProgrammatically)
	if failed := webPage.call(pageNavigate, uintptr(unsafe.Pointer(wide(appURL)))); failed != 0 {
		return webStepFailed("open the page", failed)
	}
	log.Printf("app window: hosted in WebView2")
	return 0
}

// webStepFailed switches to Edge. It's posted because WebView2 is still mid-call.
func webStepFailed(step string, code uint32) uintptr {
	log.Printf("app window: WebView2 couldn't %s: %#x", step, code)
	postMessage.Call(host, wmWebFallback, 0, 0)
	return 0
}

// useBrowserWindow gives up on WebView2 for this version, and opens Edge instead.
func useBrowserWindow() {
	webUnavailable = true
	if window := pageHost.Load(); window != 0 {
		destroyWindow.Call(window)
	}
	os.WriteFile(webAttempt(), nil, 0o644)
	lastLaunch = time.Time{}
	openApp()
}

func createPageWindow() bool {
	instance, _, _ := getModuleHandle.Call(0)
	name := wide(pageWindowClass)
	if !pageClassRegistered {
		brush, _, _ := getStockObject.Call(blackBrush)
		icon, _, _ := loadIcon.Call(instance, 1)
		class := windowClass{
			WndProc:    syscall.NewCallback(pageWindowProc),
			Instance:   instance,
			Icon:       icon,
			IconSm:     icon,
			Background: brush,
			ClassName:  name,
		}
		class.Size = uint32(unsafe.Sizeof(class))
		if atom, _, _ := registerClassEx.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
			return false
		}
		pageClassRegistered = true
	}
	framelessApp.Store(!appStore.Options().KeepTitleBar)
	dpi, _, _ := getDpiForSystem.Call()
	width, height := pageWidth*dpi/baseDPI, pageHeight*dpi/baseDPI
	window, _, _ := createWindowEx.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(wide(appName))), wsOverlappedWindow,
		useDefaultPosition, useDefaultPosition, width, height, 0, 0, instance, 0)
	pageHost.Store(window)
	if window == 0 {
		return false
	}
	// Windows doesn't ask for the frame's size properly until it's told the frame changed.
	setWindowPos.Call(window, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
	showWindow.Call(window, swShowNormal)
	setForegroundWindow.Call(window)
	return true
}

// pageLoaded runs when the page has drawn itself and said which frame it wants.
func pageLoaded(frameless bool) {
	webLoaded.Store(true)
	os.Remove(webAttempt())
	if framelessApp.Swap(frameless) != frameless {
		setWindowPos.Call(pageHost.Load(), 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpNoActivate|swpFrameChanged)
	}
}

// fitPage makes the page fill the window.
func fitPage(window uintptr) {
	var box windowBox
	if webController != nil {
		getClientRect.Call(window, uintptr(unsafe.Pointer(&box)))
		webController.call(controllerPutBounds, uintptr(unsafe.Pointer(&box)))
	}
}

// trimFrame removes the title bar while keeping the invisible edges used for resizing.
func trimFrame(window uintptr, box *windowBox) {
	dpi, _, _ := getDpiForWindow.Call(window)
	padding, _, _ := getSystemMetricDPI.Call(smCxPaddedBorder, dpi)
	frameX, _, _ := getSystemMetricDPI.Call(smCxFrame, dpi)
	frameY, _, _ := getSystemMetricDPI.Call(smCyFrame, dpi)
	x, y := int32(frameX+padding), int32(frameY+padding)
	box.Left += x
	box.Right -= x
	box.Bottom -= y
	if zoomed, _, _ := isZoomed.Call(window); zoomed != 0 {
		box.Top += y // A maximized window hangs off the screen by this much.
	}
}

func closeHostedPage() {
	pageHost.Store(0)
	os.Remove(webAttempt())
	if webController != nil {
		webController.call(controllerClose)
		webController.call(comRelease)
		webController = nil
	}
	if webPage != nil {
		webPage.call(comRelease)
		webPage = nil
	}
	if webEnvironment != nil {
		webEnvironment.call(comRelease) // Lets the browser processes end.
		webEnvironment = nil
	}
}

func pageWindowProc(window, msg, wParam uintptr, lParam unsafe.Pointer) uintptr {
	switch uint32(msg) {
	case wmNcCalcSize:
		if wParam != 0 && framelessApp.Load() {
			trimFrame(window, (*windowBox)(lParam))
			return 0
		}
	case wmSize:
		// A minimized page stops drawing, so it costs a game nothing.
		showing := wParam != sizeMinimized
		if webController != nil && showing != pageShowing {
			pageShowing = showing
			var value uintptr
			if showing {
				value = 1
			}
			webController.call(controllerPutVisible, value)
		}
		if showing {
			fitPage(window)
		}
	case wmMove:
		if webController != nil {
			webController.call(controllerParentMoved)
		}
	case wmSetFocus:
		if webController != nil {
			webController.call(controllerMoveFocus, moveFocusProgrammatically)
		}
	case wmDpiChanged:
		box := (*windowBox)(lParam)
		setWindowPos.Call(window, 0, uintptr(uint32(box.Left)), uintptr(uint32(box.Top)), uintptr(box.Right-box.Left), uintptr(box.Bottom-box.Top), swpNoZOrder|swpNoActivate)
		return 0
	case wmDestroy:
		closeHostedPage()
		return 0
	}
	result, _, _ := defWindowProc.Call(window, msg, wParam, uintptr(lParam))
	return result
}
