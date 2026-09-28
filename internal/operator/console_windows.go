//go:build windows

package operator

import "syscall"

// setConsoleUTF8 переключает кодовую страницу консоли Windows на UTF-8.
func setConsoleUTF8() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("SetConsoleOutputCP")
	_, _, _ = proc.Call(65001)
}