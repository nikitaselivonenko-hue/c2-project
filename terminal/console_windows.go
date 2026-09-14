//go:build windows

package main

import "syscall"

func setConsoleUTF8() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("SetConsoleOutputCP")
	proc.Call(65001)
}