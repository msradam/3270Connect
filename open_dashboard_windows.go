//go:build windows
// +build windows

package main

import (
	"os"

	"github.com/jchv/go-webview2"
	"github.com/pterm/pterm"
)

func openDashboardEmbedded() {
	if !*startDashboard {
		pterm.Warning.Println("Dashboard mode not enabled. Skipping embedded browser launch.")
		return
	}

	debug := false
	w := webview2.New(debug)
	defer func() {
		if r := recover(); r != nil {
			pterm.Error.Println("Recovered from a panic in openDashboardEmbedded:", r)
		}
		w.Destroy()
	}()

	w.SetTitle("3270Connect Dashboard")
	w.SetSize(1024, 768, webview2.HintNone)

	iconPath := "logo.png"
	if _, err := os.Stat(iconPath); err == nil {
		pterm.Info.Printf("Icon file %s found. Proceeding without setting icon (not supported in webview2).\n", iconPath)
	} else {
		pterm.Warning.Printf("Icon file %s not found. Skipping icon setup.\n", iconPath)
	}

	w.Navigate("http://localhost:9200/dashboard")

	defer func() {
		pterm.Info.Println("WebView2 window closed. Initiating shutdown.")
		pid := os.Getpid()
		proc, err := os.FindProcess(pid)
		if err != nil {
			pterm.Error.Printf("Failed to find process with PID %d: %v\n", pid, err)
			return
		}
		if err := proc.Kill(); err != nil {
			pterm.Error.Printf("Failed to terminate process with PID %d: %v\n", pid, err)
		} else {
			pterm.Success.Printf("Process with PID %d terminated successfully.\n", pid)
		}
	}()

	w.Run()
}
