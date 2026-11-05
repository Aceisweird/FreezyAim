//go:build windows

// Windows features behind Settings: global shortcuts and starting with Windows.
package main

import (
	"errors"
	"os"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	hotkeyToggle = 1
	hotkeyNext   = 2
	modNoRepeat  = 0x4000 // Holding a key down only counts once.

	runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
)

var (
	advapi32       = syscall.NewLazyDLL("advapi32.dll")
	regDeleteValue = advapi32.NewProc("RegDeleteValueW")
	regSetValue    = advapi32.NewProc("RegSetValueExW")

	registerHotKey   = user32.NewProc("RegisterHotKey")
	unregisterHotKey = user32.NewProc("UnregisterHotKey")
	sendMessage      = user32.NewProc("SendMessageW")

	// pendingKeys carries new shortcuts to the main thread, which must register them.
	pendingKeys atomic.Pointer[[2]Hotkey]

	appStore *Store
)

func registerKey(id uintptr, key Hotkey) bool {
	unregisterHotKey.Call(host, id)
	if key.Key == 0 {
		return true
	}
	ok, _, _ := registerHotKey.Call(host, id, uintptr(key.Modifiers)|modNoRepeat, uintptr(key.Key))
	return ok != 0
}

// applyHotkeys must run on the main thread.
func applyHotkeys(toggle, next Hotkey) bool {
	toggled := registerKey(hotkeyToggle, toggle)
	moved := registerKey(hotkeyNext, next)
	return toggled && moved
}

// ApplyHotkeys waits for the main thread, so it can report a shortcut that's taken.
func (windowsOverlay) ApplyHotkeys(toggle, next Hotkey) error {
	pendingKeys.Store(&[2]Hotkey{toggle, next})
	if failed, _, _ := sendMessage.Call(host, wmSetHotkeys, 0, 0); failed != 0 {
		return errors.New("Another app is using that shortcut. Try a different one.")
	}
	return nil
}

func (windowsOverlay) StartsWithWindows() bool {
	var key syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, wide(runKey), 0, syscall.KEY_QUERY_VALUE, &key) != nil {
		return false
	}
	defer syscall.RegCloseKey(key)
	return syscall.RegQueryValueEx(key, wide(appName), nil, nil, nil, nil) == nil
}

func (windowsOverlay) SetStartWithWindows(on bool) error {
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, wide(runKey), 0, syscall.KEY_SET_VALUE, &key); err != nil {
		return err
	}
	defer syscall.RegCloseKey(key)
	if !on {
		regDeleteValue.Call(uintptr(key), uintptr(unsafe.Pointer(wide(appName))))
		return nil
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	command, err := syscall.UTF16FromString(`"` + executable + `"`)
	if err != nil {
		return err
	}
	const stringValue = 1 // REG_SZ
	failed, _, _ := regSetValue.Call(uintptr(key), uintptr(unsafe.Pointer(wide(appName))), 0, stringValue,
		uintptr(unsafe.Pointer(&command[0])), uintptr(len(command)*2))
	if failed != 0 {
		return syscall.Errno(failed)
	}
	return nil
}
