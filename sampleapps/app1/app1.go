package app1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/racingmars/go3270"
)

func init() {
	// put the go3270 library in debug mode
	//go3270.Debug = os.Stderr
	// Set up pterm with a funky theme
	pterm.DefaultSection.Style = pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	pterm.Info.Prefix = pterm.Prefix{Text: "INFO", Style: pterm.NewStyle(pterm.BgBlue, pterm.FgWhite)}
	pterm.Error.Prefix = pterm.Prefix{Text: "ERROR", Style: pterm.NewStyle(pterm.BgRed, pterm.FgWhite)}
	pterm.Success.Prefix = pterm.Prefix{Text: "SUCCESS", Style: pterm.NewStyle(pterm.BgGreen, pterm.FgBlack)}
	pterm.Warning.Prefix = pterm.Prefix{Text: "WARNING", Style: pterm.NewStyle(pterm.BgYellow, pterm.FgBlack)}

}

var screen1 = go3270.Screen{
	{Row: 0, Col: 27, Intense: true, Content: "3270 Example Application"},
	{Row: 2, Col: 0, Content: "Welcome to the go3270 example application. Please enter your name."},
	{Row: 4, Col: 0, Content: "First Name  . . ."},
	{Row: 4, Col: 19, Name: "fname", Write: true, Highlighting: go3270.Underscore},
	{Row: 4, Col: 40, Autoskip: true}, // field "stop" character
	{Row: 5, Col: 0, Content: "Last Name . . . ."},
	{Row: 5, Col: 19, Name: "lname", Write: true, Highlighting: go3270.Underscore},
	{Row: 5, Col: 40, Autoskip: true}, // field "stop" character
	{Row: 6, Col: 0, Content: "Password  . . . ."},
	{Row: 6, Col: 19, Name: "password", Write: true, Hidden: true},
	{Row: 6, Col: 40}, // field "stop" character
	{Row: 8, Col: 0, Content: "Press"},
	{Row: 8, Col: 6, Intense: true, Content: "enter"},
	{Row: 8, Col: 12, Content: "to submit your name."},
	{Row: 10, Col: 0, Intense: true, Color: go3270.Red, Name: "errormsg"}, // a blank field for error messages
	{Row: 22, Col: 0, Content: "PF3 Exit"},
}

var screen2 = go3270.Screen{
	{Row: 0, Col: 27, Intense: true, Content: "3270 Example Application"},
	{Row: 2, Col: 0, Content: "Thank you for submitting your name. Here's what I know:"},
	{Row: 4, Col: 0, Content: "Your first name is"},
	{Row: 4, Col: 19, Name: "fname"}, // We're giving this field a name to replace its value at runtime
	{Row: 5, Col: 0, Content: "And your last name is"},
	{Row: 5, Col: 22, Name: "lname"}, // We're giving this field a name to replace its value at runtime
	{Row: 6, Col: 0, Name: "passwordOutput"},
	{Row: 8, Col: 0, Content: "Press"},
	{Row: 8, Col: 6, Intense: true, Content: "enter"},
	{Row: 8, Col: 12, Content: "to enter your name again, or"},
	{Row: 8, Col: 41, Intense: true, Content: "PF3"},
	{Row: 8, Col: 45, Content: "to quit and disconnect."},
	{Row: 11, Col: 0, Color: go3270.Turquoise, Highlighting: go3270.ReverseVideo, Content: "Here is a field with extended attributes."},
	{Row: 11, Col: 42}, // remember to "stop" fields with a regular field, to clear the reverse video for example
	{Row: 22, Col: 0, Content: "PF3 Exit"},
}

// Add Metrics type for dashboard compatibility
type Metrics struct {
	PID                     int       `json:"pid"`
	ActiveWorkflows         int       `json:"activeWorkflows"`
	TotalWorkflowsStarted   int       `json:"totalWorkflowsStarted"`
	TotalWorkflowsCompleted int       `json:"totalWorkflowsCompleted"`
	TotalWorkflowsFailed    int       `json:"totalWorkflowsFailed"`
	Durations               []float64 `json:"durations"`
	CPUUsage                []float64 `json:"cpuUsage"`
	MemoryUsage             []float64 `json:"memoryUsage"`
	Params                  string    `json:"params"`
	RuntimeDuration         int       `json:"runtimeDuration"`
	StartTimestamp          int64     `json:"startTimestamp"`
}

// startMetricsUpdater periodically writes a minimal metrics file.
func startMetricsUpdater() {
	pid := os.Getpid()
	for {
		metrics := Metrics{
			PID:                     pid,
			ActiveWorkflows:         0,
			TotalWorkflowsStarted:   0,
			TotalWorkflowsCompleted: 0,
			TotalWorkflowsFailed:    0,
			Durations:               []float64{},
			CPUUsage:                []float64{},
			MemoryUsage:             []float64{},
			Params:                  "-runApp 1",
			RuntimeDuration:         0,
			StartTimestamp:          time.Now().Unix(),
		}
		dir, err := os.UserConfigDir()
		if err != nil {
			dir = filepath.Join(".", "dashboard")
		} else {
			dir = filepath.Join(dir, "3270Connect", "dashboard")
		}
		os.MkdirAll(dir, 0755)
		data, _ := json.Marshal(metrics)
		ioutil.WriteFile(filepath.Join(dir, fmt.Sprintf("metrics_%d.json", pid)), data, 0644)
		time.Sleep(5 * time.Second)
	}
}

// In RunApplication, start the metrics updater before accepting connections.
func RunApplication(port int) {
	go startMetricsUpdater()
	address := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
	defer ln.Close()

	pterm.Info.Printf("Listening on port %d for connections\n", port)
	pterm.Info.Printf("Press Ctrl-C to end server.")

	for {
		conn, err := ln.Accept()
		if err != nil {
			pterm.Error.Printf("Error accepting connection: %v", err)
			continue
		}
		go handle(conn)
	}
}

// handle is the handler for individual user connections.
func handle(conn net.Conn) {
	defer conn.Close()

	// Always begin new connection by negotiating the telnet options
	go3270.NegotiateTelnet(conn)

	fieldValues := make(map[string]string)

	// We will loop forever until the user quits with PF3
mainLoop:
	for {
	screen1Loop:
		for {
			// loop until the user passes input validation, or quits

			// Always reset password input to blank each time through the loop
			fieldValues["password"] = ""

			// Show the first screen, and wait to get a client response. Place
			// the cursor at the beginning of the first input field.
			// We're passing in the fieldValues map to carry values over from
			// the previous submission. We could pass nil, instead, if always want
			// the fields to start out blank.
			response, err := go3270.ShowScreen(screen1, fieldValues, 4, 20, conn)
			if err != nil {
				//pterm.Error.Printf("%v", err)
				return
			}

			// If the user pressed PF3, exit
			if response.AID == go3270.AIDPF3 {
				break mainLoop
			}

			// If anything OTHER than "Enter", restart the loop
			if response.AID != go3270.AIDEnter {
				continue screen1Loop
			}

			// User must have pressed "Enter", so let's check the input.
			fieldValues = response.Values
			if strings.TrimSpace(fieldValues["fname"]) == "" &&
				strings.TrimSpace(fieldValues["lname"]) == "" {
				fieldValues["errormsg"] = "First and Last Name fields are required."
				continue screen1Loop
			}
			if strings.TrimSpace(fieldValues["fname"]) == "" {
				fieldValues["errormsg"] = "First Name field is required."
				continue screen1Loop
			}
			if strings.TrimSpace(fieldValues["lname"]) == "" {
				fieldValues["errormsg"] = "Last Name field is required."
				continue screen1Loop
			}

			// At this point, we know the user provided both fields and had
			// hit enter, so we are going to reset the error message for the
			// next time through the loop, and break out of this loop so we
			// move on to screen 2.
			fieldValues["errormsg"] = ""
			break screen1Loop
		}

		// Now we're ready to display screen2
		passwordLength := len(strings.TrimSpace(fieldValues["password"]))
		passwordPlural := "s"
		if passwordLength == 1 {
			passwordPlural = ""
		}
		fieldValues["passwordOutput"] = fmt.Sprintf("Your password was %d character%s long",
			passwordLength, passwordPlural)
		response, err := go3270.ShowScreen(screen2, fieldValues, 0, 0, conn)
		if err != nil {
			//pterm.Error.Printf("%v", err)
			return
		}

		// If the user pressed PF3, exit
		if response.AID == go3270.AIDPF3 {
			break
		}

		// If they pressed anything else, just let the loop continue...
		continue
	}

	pterm.Success.Println("Connection closed")
}
