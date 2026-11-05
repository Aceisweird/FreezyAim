//go:build windows

// Controls for the app window. When the page is in a browser window, that title bar is
// drawn by the browser and can't be turned off, so it's trimmed from the visible window.
package main

import (
	"log"
	"sync/atomic"
	"time"
	"unsafe"
)

const browserPageClass = "Chrome_RenderWidgetHostHWND" // The child window holding the page.

const (
	swMaximize   = 3
	swMinimize   = 6
	vkLeftButton = 0x01
	cornerRadius = 16
	maxBarHeight = 120 // Physical pixels, so a bad measurement can't hide the app.
	dragPause    = 8 * time.Millisecond
	trimCheck    = 400 * time.Millisecond
)

var (
	setWindowRgn       = user32.NewProc("SetWindowRgn")
	getWindowRect      = user32.NewProc("GetWindowRect")
	getAsyncKeyState   = user32.NewProc("GetAsyncKeyState")
	isZoomed           = user32.NewProc("IsZoomed")
	getWindowRgn       = user32.NewProc("GetWindowRgn")
	createRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	createRectRgn      = gdi32.NewProc("CreateRectRgn")

	trimmedBar   atomic.Int32 // The height being trimmed. 0 means the bar is showing.
	trimWatching atomic.Bool
)

type windowBox struct{ Left, Top, Right, Bottom int32 }

func appWindow() uintptr {
	if hosted := pageHost.Load(); hosted != 0 {
		return hosted
	}
	window, _, _ := findWindow.Call(uintptr(unsafe.Pointer(wide(browserWindowClass))), uintptr(unsafe.Pointer(wide(appName))))
	return window
}

// watchTrim puts the trim back, because the browser can clear it by itself.
func watchTrim() {
	defer trimWatching.Store(false)
	reported := false
	ticker := time.NewTicker(trimCheck)
	defer ticker.Stop()
	for range ticker.C {
		bar := trimmedBar.Load()
		window := appWindow()
		if window == 0 {
			return // The window closed, so nothing here needs to run while gaming.
		}
		if bar == 0 {
			continue
		}
		if top := pageTop(window); top > 0 && top <= maxBarHeight && int32(top) != bar {
			trimWindow(window, top)
			continue
		}
		probe, _, _ := createRectRgn.Call(0, 0, 0, 0)
		kind, _, _ := getWindowRgn.Call(window, probe)
		deleteObject.Call(probe)
		if kind == 0 {
			if !reported {
				log.Printf("title bar: the browser cleared the trim, so it was applied again")
				reported = true
			}
			trimWindow(window, int(bar))
		}
	}
}

func (windowsOverlay) Window(action string, barHeight int) {
	if pageHost.Load() != 0 && (action == "trim" || action == "restore") {
		pageLoaded(action == "trim")
		return
	}
	window := appWindow()
	if window == 0 {
		log.Printf("title bar: couldn't find the app window for %q", action)
		return
	}
	switch action {
	case "trim":
		trimWindow(window, barHeight)
		if trimWatching.CompareAndSwap(false, true) {
			go watchTrim()
		}
	case "restore":
		trimmedBar.Store(0)
		setWindowRgn.Call(window, 0, 1)
	case "minimize":
		showWindow.Call(window, swMinimize)
	case "maximize":
		if zoomed, _, _ := isZoomed.Call(window); zoomed != 0 {
			showWindow.Call(window, swRestore)
		} else {
			showWindow.Call(window, swMaximize)
		}
	case "close":
		postMessage.Call(window, wmClose, 0, 0)
	case "drag":
		go dragWindow(window)
	}
}

// pageTop finds how far down the window the page starts, measured from the browser's
// own child windows. It's exact on any display, and 0 if the browser has no such window.
func pageTop(window uintptr) int {
	class := wide(browserPageClass)
	top, tallest := 0, int32(0)
	child := uintptr(0)
	for {
		child, _, _ = findWindowEx.Call(window, child, uintptr(unsafe.Pointer(class)), 0)
		if child == 0 {
			return top
		}
		var outer, inner windowBox
		getWindowRect.Call(window, uintptr(unsafe.Pointer(&outer)))
		getWindowRect.Call(child, uintptr(unsafe.Pointer(&inner)))
		if height := inner.Bottom - inner.Top; height > tallest {
			tallest, top = height, int(inner.Top-outer.Top)
		}
	}
}

// trimWindow shows only the part of the window below the title bar.
func trimWindow(window uintptr, barHeight int) {
	var box windowBox
	if top := pageTop(window); top > 0 && top != barHeight {
		log.Printf("title bar: the page starts at %d pixels, the app measured %d", top, barHeight)
		barHeight = top
	}
	if barHeight <= 0 || barHeight > maxBarHeight {
		log.Printf("title bar: ignored a height of %d", barHeight)
		return
	}
	trimmedBar.Store(int32(barHeight))
	getWindowRect.Call(window, uintptr(unsafe.Pointer(&box)))
	region, _, _ := createRoundRectRgn.Call(0, uintptr(barHeight), uintptr(box.Right-box.Left+1), uintptr(box.Bottom-box.Top+1), cornerRadius, cornerRadius)
	if region != 0 {
		done, _, _ := setWindowRgn.Call(window, region, 1) // Windows owns the region from here.
		log.Printf("title bar: trimmed %d pixels from a %d by %d window, result %d", barHeight, box.Right-box.Left, box.Bottom-box.Top, done)
	}
}

// dragWindow moves the window with the pointer until the mouse button is released.
func dragWindow(window uintptr) {
	var box windowBox
	var start, now point
	if zoomed, _, _ := isZoomed.Call(window); zoomed != 0 {
		return
	}
	getWindowRect.Call(window, uintptr(unsafe.Pointer(&box)))
	getCursorPos.Call(uintptr(unsafe.Pointer(&start)))
	for {
		held, _, _ := getAsyncKeyState.Call(vkLeftButton)
		if held&0x8000 == 0 {
			return
		}
		getCursorPos.Call(uintptr(unsafe.Pointer(&now)))
		x, y := box.Left+now.X-start.X, box.Top+now.Y-start.Y
		setWindowPos.Call(window, 0, uintptr(uint32(x)), uintptr(uint32(y)), 0, 0, swpNoSize|swpNoActivate|swpNoZOrder)
		time.Sleep(dragPause)
	}
}
