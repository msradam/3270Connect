//go:build !windows
// +build !windows

package main

func openDashboardEmbedded() {
	pterm.Warning.Println("Embedded dashboard is only supported on Windows.")
}
