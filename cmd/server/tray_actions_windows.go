//go:build windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"nudge/internal/clipboard"
	"nudge/internal/pair"
)

// Actions behind the tray menu items. They run on the tray thread (inside
// the menu's WM_ handling) unless they could block, in which case they run
// in a goroutine and only log the outcome.

var (
	procShellExecuteW   = shell32.NewProc("ShellExecuteW")
	procSHGetFolderPath = shell32.NewProc("SHGetFolderPathW")
)

const (
	csidlStartup         = 0x0007
	swShowNormal         = 1
	createNoWindow       = 0x08000000
	autostartScript      = "toggle-autostart.ps1"
	maxPathChars         = 260
	shellExecuteMaxError = 32 // ShellExecuteW returns > 32 on success
)

// autostartTimeout bounds one toggle run (a var so tests can shorten it).
var autostartTimeout = 30 * time.Second

// runAutostartScript runs the toggle script for state "on"/"off" and returns
// when it ends or ctx is cancelled (which kills it). A var so tests can
// replace it.
var runAutostartScript = func(ctx context.Context, state string) error {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-File", autostartScriptPath(), state, "-NoPause")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// startupDir is the user's Startup folder (honours folder redirection).
func startupDir() (string, bool) {
	if err := procSHGetFolderPath.Find(); err != nil {
		return "", false
	}
	buf := make([]uint16, maxPathChars)
	r, _, _ := procSHGetFolderPath.Call(0, csidlStartup, 0, 0, uintptr(unsafe.Pointer(&buf[0])))
	if r != 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

// autostartState reports whether the Startup shortcut exists and whether
// the toggle script (installed next to the exe) is available.
func autostartState() (on, available bool) {
	dir, ok := startupDir()
	if !ok {
		return false, false
	}
	if _, err := os.Stat(autostartLinkPath(dir)); err == nil {
		on = true
	}
	_, err := os.Stat(autostartScriptPath())
	return on, err == nil
}

func autostartScriptPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), autostartScript)
}

// setAutostart runs the installed toggle script hidden; the .lnk is created
// with the same target, icon and description as the installer's. It returns
// false (doing nothing) when a toggle is already in flight.
func (t *winTray) setAutostart(on bool) bool {
	state := "off"
	if on {
		state = "on"
	}
	// Single owner of the busy flag: tray menu and panel both come through
	// here, so overlapping toggles are refused whoever calls.
	if !t.autostartBusy.CompareAndSwap(false, true) {
		t.logf("autostart %s ignored: a toggle is already running", state)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), autostartTimeout)
	go func() {
		defer t.autostartBusy.Store(false)
		defer cancel()
		err := runAutostartScript(ctx, state)
		switch {
		case ctx.Err() == context.DeadlineExceeded:
			t.logf("autostart %s timed out after %v and was killed: %v", state, autostartTimeout, err)
		case err != nil:
			t.logf("autostart %s failed: %v", state, err)
		default:
			t.logf("autostart %s", state)
		}
	}()
	return true
}

func (t *winTray) copyCode() {
	_, code := t.broker.Verification()
	if code == "" {
		return
	}
	if err := clipboard.SetText(code); err != nil {
		t.logf("copying the verification code failed: %v", err)
	}
}

func (t *winTray) openLogFolder() {
	shellOpen(t.hwnd.Load(), pair.ConfigDir(), t.logf)
}

func shellOpen(hwnd uintptr, target string, logf func(string, ...any)) {
	verb, _ := syscall.UTF16PtrFromString("open")
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return
	}
	r, _, callErr := procShellExecuteW.Call(hwnd, uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)), 0, 0, swShowNormal)
	if r <= shellExecuteMaxError {
		logf("opening %s failed: %v", target, callErr)
	}
}

// revoke asks for confirmation (default No), then revokes the phone.
func (t *winTray) revoke(deviceID string) {
	devices, _ := t.broker.Snapshot()
	name, ok := trayDeviceName(devices, deviceID)
	if !ok {
		return // already gone
	}
	title, text := trayRevokeConfirm(name)
	if messageBox(title, text) != idYes {
		return
	}
	if err := t.broker.Revoke(deviceID); err != nil {
		t.logf("revoke %s failed: %v", shortID(deviceID), err)
		return
	}
	t.logf("revoked device %s from the tray", shortID(deviceID))
}
