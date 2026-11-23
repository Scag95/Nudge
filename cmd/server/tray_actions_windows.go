//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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
	csidlStartup     = 0x0007
	swShowNormal     = 1
	createNoWindow   = 0x08000000
	autostartScript  = "toggle-autostart.ps1"
	maxPathChars     = 260
	shellExecuteOKAt = 32 // ShellExecuteW returns > 32 on success
)

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
// with the same target, icon and description as the installer's.
func (t *winTray) setAutostart(on bool) {
	state := "off"
	if on {
		state = "on"
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-File", autostartScriptPath(), state, "-NoPause")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	t.autostartBusy.Store(true)
	go func() {
		defer t.autostartBusy.Store(false)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.logf("autostart %s failed: %v: %s", state, err, out)
			return
		}
		t.logf("autostart %s", state)
	}()
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
	if r <= shellExecuteOKAt {
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
