//go:build !windows
// +build !windows

package main

import "github.com/pterm/pterm"

func openDashboardEmbedded() {
	pterm.Warning.Println("Embedded dashboard is only supported on Windows.")
}
