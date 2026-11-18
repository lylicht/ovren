package platform

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func IsAdmin() bool { return os.Geteuid() == 0 }

const lockPath = "/var/run/OvrenNET.lock"

var lockFile *os.File

func SingleInstance() bool {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return false
	}
	if unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		f.Close()
		return false
	}
	lockFile = f
	return true
}

func InstanceRunning() bool {
	f, err := os.Open(lockPath)
	if err != nil {
		return false
	}
	defer f.Close()
	if unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB) != nil {
		return true
	}
	unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return false
}

func RedirectStd(f *os.File) {
	os.Stdout, os.Stderr = f, f
	log.SetOutput(f)
	unix.Dup2(int(f.Fd()), 1)
	unix.Dup2(int(f.Fd()), 2)
}

const (
	exePreferred = "/usr/local/bin/OvrenNET"
	exeFallback  = "/opt/OvrenNET/bin/OvrenNET"
)

func InstalledExe() string {
	if _, err := os.Stat(exeFallback); err == nil {
		return exeFallback
	}
	return exePreferred
}

func RelaunchElevated(args []string, hidden bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if HasConsole() {
		return syscall.Exec("/usr/bin/sudo", append([]string{"sudo", exe}, args...), os.Environ())
	}
	cmd := shQuote(exe)
	for _, a := range args {
		cmd += " " + shQuote(a)
	}

	script := `do shell script "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(cmd+" >/dev/null 2>&1 &") + `" with administrator privileges`
	if out, err := exec.Command("/usr/bin/osascript", "-e", script).CombinedOutput(); err != nil {
		return fmt.Errorf("administrator prompt failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func RelaunchTray(args []string, hidden bool) error {
	if len(args) == 1 && args[0] == "tray" {
		if exe := InstalledExe(); exec.Command("/usr/bin/sudo", "-n", "-l", exe, "tray").Run() == nil {
			return syscall.Exec("/usr/bin/sudo", []string{"sudo", "-n", exe, "tray"}, os.Environ())
		}
	}
	return RelaunchElevated(args, hidden)
}

func shQuote(s string) string { return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'` }

func HasConsole() bool {
	_, err := unix.IoctlGetTermios(2, unix.TIOCGETA)
	return err == nil
}

func ShowError(msg string) {
	exec.Command("/usr/bin/osascript",
		"-e", "on run argv", "-e", `display notification (item 1 of argv) with title "OvrenNET"`, "-e", "end run",
		msg).Run()
}

func CheckDriver() error { return nil }

func WaitForTray(max time.Duration) bool { return true }

func signInUser() (string, error) {
	uid, err := exec.Command("/usr/bin/stat", "-f", "%u", "/dev/console").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(uid)), nil
}

func OpenURL(url string) error {
	if uid, err := signInUser(); err == nil && uid != "" {
		if exec.Command("/bin/launchctl", "asuser", uid, "open", url).Run() == nil {
			return nil
		}
	}
	cmd := exec.Command("open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
