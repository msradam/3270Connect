package app2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
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

// Predefined feed URLs
const (
	skyNewsFeedURL   = "https://feeds.skynews.com/feeds/rss/uk.xml"
	metOfficeFeedURL = "https://www.metoffice.gov.uk/public/data/PWSCache/WarningsRSS/Region/UK"
	ncscFeedURL      = "https://www.ncsc.gov.uk/api/1/services/v1/all-rss-feed.xml"
	bbcFeedURL       = "https://feeds.bbci.co.uk/news/rss.xml"
)

var feedSelectionScreen = go3270.Screen{
	{Row: 0, Col: 27, Intense: true, Content: "RSS Newsreader Application"},
	{Row: 2, Col: 0, Content: "Select the RSS feed to view:"},
	{Row: 4, Col: 0, Content: "(1) Sky UK News"},
	{Row: 5, Col: 0, Content: "(2) Met Office UK Weather"},
	{Row: 6, Col: 0, Content: "(3) NCSC Latest"},
	{Row: 7, Col: 0, Content: "(4) BBC Top Stories"},
	//{Row: 9, Col: 0, Content: "Enter the number of your choice and press enter."},
	{Row: 10, Col: 0, Content: "Choice:"},
	{Row: 10, Col: 8, Name: "feedChoice", Write: true, Highlighting: go3270.Underscore},
	{Row: 10, Col: 11, Autoskip: true}, // field "stop" character
	{Row: 22, Col: 0, Content: "PF3 Exit"},
}

// This is a simplified screen for displaying headlines; in a real application, you would need to handle scrolling and selection.
var headlinesScreen = go3270.Screen{
	{Row: 0, Col: 27, Intense: true, Content: "Headlines"},
	// Placeholder for headlines; these would be populated at runtime
	{Row: 2, Col: 0, Content: "Headline 1"},
	{Row: 3, Col: 0, Content: "Headline 2"},
	// ...
	{Row: 22, Col: 0, Content: "PF3 Back"},
}

func fetchRSSFeed(url string) ([]*gofeed.Item, error) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return feed.Items, nil
}

func displayHeadlines(conn net.Conn, items []*gofeed.Item) (string, error) {
	const startRow = 2
	const maxItems = 15 // Maximum number of items to display

	// Build the dynamic screen with actual headlines
	dynamicHeadlinesScreen := make(go3270.Screen, 0, maxItems+4)

	// Title for the headlines
	dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
		Row: 0, Col: 0, Intense: true, Content: "Headlines",
	})

	// Populate the screen with headlines
	for i, item := range items {
		if i >= maxItems {
			break
		}
		dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
			Row: startRow + i, Col: 0, Content: fmt.Sprintf("%d. %s", i+1, item.Title),
		})
	}

	// Input field for the selection
	dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
		Row: maxItems + 3, Col: 0, Content: "Choice:",
	})

	dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
		Row: maxItems + 3, Col: 8, Name: "selection", Write: true, Highlighting: go3270.Underscore,
	})

	dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
		Row: maxItems + 3, Col: 11, Autoskip: true,
	})

	dynamicHeadlinesScreen = append(dynamicHeadlinesScreen, go3270.Field{
		Row: maxItems + 5, Col: 0, Content: "PF3 Exit",
	})

	// Show the screen and wait for input
	response, err := go3270.ShowScreen(dynamicHeadlinesScreen, nil, maxItems+3, 9, conn)
	if err != nil {
		return "", err // Return an empty string and error if something goes wrong
	}

	// Check if the user wants to exit the headline selection screen
	if response.AID == go3270.AIDPF3 {
		return "PF3", nil
	}

	// Return the selection value
	return strings.TrimSpace(response.Values["selection"]), nil
}

func displayDetails(conn net.Conn, item *gofeed.Item) {
	// Calculate the number of rows needed for the description
	// Assuming we can fit approximately 80 characters per row
	descRows := len(item.Description) / 80
	if len(item.Description)%80 != 0 {
		descRows++ // Add an extra row for any remaining characters
	}

	// Create a new screen slice with enough rows for the title, description, and footer
	detailsScreen := make(go3270.Screen, 2+descRows+1) // +1 for the footer

	// Title row
	detailsScreen[0] = go3270.Field{Row: 0, Col: 0, Content: "Title: " + item.Title, Intense: true}

	// Description rows
	desc := item.Description
	for i := 0; i < descRows; i++ {
		// Extract a substring for each row
		startIdx := i * 79
		endIdx := startIdx + 79
		if endIdx > len(desc) {
			endIdx = len(desc)
		}

		detailsScreen[i+1] = go3270.Field{Row: i + 2, Col: 0, Content: desc[startIdx:endIdx]}
	}

	// Footer row
	detailsScreen[2+descRows] = go3270.Field{Row: 22, Col: 0, Content: "PF3 - Return"}

	// Wait for the user to press PF3 to return to the headlines
	for {
		response, err := go3270.ShowScreen(detailsScreen, nil, 0, 0, conn)
		if err != nil {
			pterm.Error.Printf("Error waiting for user action: %v", err)
			return
		}
		if response.AID == go3270.AIDPF3 {
			break // User pressed PF3, return to the headlines list
		}
	}
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
			TotalWorkflowsStarted:   1,
			TotalWorkflowsCompleted: 0,
			TotalWorkflowsFailed:    0,
			Durations:               []float64{},
			CPUUsage:                []float64{},
			MemoryUsage:             []float64{},
			Params:                  "-runApp 2",
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

func handle(conn net.Conn) {
	defer conn.Close()
	go3270.NegotiateTelnet(conn)

	var items []*gofeed.Item
	//var err error

	for {
		response, err := go3270.ShowScreen(feedSelectionScreen, nil, 10, 9, conn)
		if err != nil {
			pterm.Error.Printf("Error displaying feed selection screen: %v", err)
			return
		}

		if response.AID == go3270.AIDPF3 {
			return // Exit if PF3 is pressed
		}

		if response.AID == go3270.AIDEnter {
			feedChoice := strings.TrimSpace(response.Values["feedChoice"])
			var feedURL string

			switch feedChoice {
			case "1":
				feedURL = skyNewsFeedURL
			case "2":
				feedURL = metOfficeFeedURL
			case "3":
				feedURL = ncscFeedURL
			case "4":
				feedURL = bbcFeedURL
			default:
				fmt.Println("Invalid selection.")
				continue
			}

			items, err = fetchRSSFeed(feedURL)
			if err != nil {
				pterm.Error.Printf("Error fetching RSS feed: %v", err)
				continue
			}

			// Loop to handle user's headline selection
			for {
				selection, err := displayHeadlines(conn, items)
				if err != nil {
					pterm.Error.Printf("Error displaying headlines: %v", err)
					break
				}

				if selection == "PF3" {
					break // User pressed PF3, go back to feed selection
				}

				selectedIndex, err := strconv.Atoi(selection)
				if err != nil || selectedIndex < 1 || selectedIndex > len(items) {
					pterm.Error.Printf("Invalid selection. Please try again.")
					continue
				}

				selectedItem := items[selectedIndex-1]
				displayDetails(conn, selectedItem)

				// After viewing details, the user may press PF3 to go back to the headlines
				// This can be handled within displayDetails or here after it returns
			}
		}
	}
}

// In RunApplication, start the metrics updater before listening for connections.
func RunApplication(port int) {
	go startMetricsUpdater()
	address := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		pterm.Error.Printf("Error starting server: %v", err)
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
