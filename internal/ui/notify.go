package ui

import (
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type rateEntry struct {
	until time.Time
}

var (
	rateMu  sync.Mutex
	rateMap = make(map[string]*rateEntry)
)

// ErrorNotify shows a Windows error dialog. Same key only once per 5 min.
func ErrorNotify(title, message string) {
	key := title + "\x00" + message
	if !rateOK(key) {
		return
	}
	msgBox(title, message, 0x00000010) // MB_ICONERROR
}

// InfoNotify shows an info dialog (not rate-limited).
func InfoNotify(title, message string) {
	msgBox(title, message, 0x00000040) // MB_ICONINFORMATION
}

func rateOK(key string) bool {
	rateMu.Lock()
	defer rateMu.Unlock()
	now := time.Now()
	if e, ok := rateMap[key]; ok && now.Before(e.until) {
		return false
	}
	rateMap[key] = &rateEntry{until: now.Add(5 * time.Minute)}
	return true
}

func msgBox(title, text string, flags uint32) {
	t, _ := windows.UTF16PtrFromString(title)
	b, _ := windows.UTF16PtrFromString(text)
	user32 := windows.NewLazySystemDLL("user32.dll")
	p := user32.NewProc("MessageBoxW")
	p.Call(0, uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(t)), uintptr(flags))
}
