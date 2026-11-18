package platform

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func IsAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

func SingleInstance() bool {
	name, _ := windows.UTF16PtrFromString(`Global\OvrenNET`)
	_, err := windows.CreateMutex(nil, false, name)
	return err != windows.ERROR_ALREADY_EXISTS
}

func InstanceRunning() bool {
	name, _ := windows.UTF16PtrFromString(`Global\OvrenNET`)
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err == nil {
		windows.CloseHandle(h)
		return true
	}
	return err == windows.ERROR_ACCESS_DENIED
}

func RedirectStd(f *os.File) {
	os.Stdout, os.Stderr = f, f
	log.SetOutput(f)
	windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(f.Fd()))
	windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
}

func RelaunchElevated(args []string, hidden bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = windows.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	show := int32(windows.SW_SHOWNORMAL)
	if hidden {
		show = windows.SW_HIDE
	}
	return windows.ShellExecute(0, verb, file, params, dir, show)
}

func RelaunchElevatedAndWait(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = windows.EscapeArg(arg)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	dir, _ := windows.UTF16PtrFromString(filepath.Dir(exe))
	info := struct {
		cbSize       uint32
		fMask        uint32
		hwnd         windows.Handle
		lpVerb       *uint16
		lpFile       *uint16
		lpParameters *uint16
		lpDirectory  *uint16
		nShow        int32
		hInstApp     windows.Handle
		lpIDList     uintptr
		lpClass      *uint16
		hkeyClass    windows.Handle
		dwHotKey     uint32
		hIcon        windows.Handle
		hProcess     windows.Handle
	}{
		fMask:        0x00000040,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		lpDirectory:  dir,
		nShow:        windows.SW_SHOWNORMAL,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	procedure := windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")
	if err := procedure.Find(); err != nil {
		return err
	}
	result, _, callErr := procedure.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		if code, ok := callErr.(syscall.Errno); ok && code != 0 {
			return callErr
		}
		return fmt.Errorf("could not launch administrator recovery")
	}
	defer windows.CloseHandle(info.hProcess)
	wait, err := windows.WaitForSingleObject(info.hProcess, windows.INFINITE)
	if err != nil {
		return err
	}
	if wait != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("waiting for administrator recovery failed: %d", wait)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("administrator recovery failed with exit code %d", exitCode)
	}
	return nil
}

func OpenURL(url string) error {
	cmd := exec.Command("explorer.exe", url)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
