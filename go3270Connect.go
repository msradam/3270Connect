package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/browser"

	connect3270 "github.com/3270io/3270Connect/connect3270"
	"github.com/3270io/3270Connect/sampleapps/app1"
	app2 "github.com/3270io/3270Connect/sampleapps/app2"

	"github.com/gin-gonic/gin"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

const version = "1.3"

var errorList []error
var errorMutex sync.Mutex

// Configuration holds the settings for the terminal connection and the steps to be executed.
type Configuration struct {
	Host            string
	Port            int
	OutputFilePath  string `json:"OutputFilePath"`
	Steps           []Step
	InputFilePath   string  `json:"InputFilePath"`
	RampUpBatchSize int     `json:"RampUpBatchSize"`
	RampUpDelay     float64 `json:"RampUpDelay"`
}

// Step represents an individual action to be taken on the terminal.
type Step struct {
	Type        string
	Coordinates connect3270.Coordinates
	Text        string
}

var (
	configFile      string
	showHelp        bool
	runAPI          bool
	apiPort         int
	concurrent      int
	headless        bool
	verbose         bool
	runApp          string
	runtimeDuration int
	lastUsedPort    int
	startPort       int
)

var dashboardStarted bool

// Global counters for metrics.
var totalWorkflowsStarted int64
var totalWorkflowsCompleted int64
var totalWorkflowsFailed int64

var dashboardPort int

var activeWorkflows int
var mutex sync.Mutex

var timingsMutex sync.Mutex
var workflowDurations []float64

var cpuHistory []float64
var memHistory []float64

var showVersion = flag.Bool("version", false, "Show the application version")

var runAppPort int

type LogEntry struct {
	PID        string    `json:"pid"`
	Parameters string    `json:"parameters"`
	Log        string    `json:"log"`
	Timestamp  time.Time `json:"timestamp"`
}

var inMemoryLogs []LogEntry
var logMutex sync.Mutex

//go:embed templates/dashboard.gohtml
var dashboardTemplateFS embed.FS

var dashboardTemplate *template.Template

func init() {
	flag.StringVar(&configFile, "config", "workflow.json", "Path to the configuration file")
	flag.BoolVar(&showHelp, "help", false, "Show usage information")
	flag.BoolVar(&runAPI, "api", false, "Run as API")
	flag.IntVar(&apiPort, "api-port", 8080, "API port")
	flag.IntVar(&concurrent, "concurrent", 1, "Number of concurrent workflows")
	flag.BoolVar(&headless, "headless", false, "Run go3270 in headless mode")
	flag.BoolVar(&verbose, "verbose", false, "Run go3270 in verbose mode")
	flag.IntVar(&runtimeDuration, "runtime", 0, "Duration to run workflows in seconds")
	flag.StringVar(&runApp, "runApp", "", "Select which sample 3270 app to run ('1' or '2')")
	flag.IntVar(&runAppPort, "runApp-port", 3270, "Port for the sample 3270 app")
	flag.IntVar(&startPort, "startPort", 5000, "Starting port for workflow connections")
	flag.IntVar(&dashboardPort, "dashboardPort", 9200, "Port for the dashboard server")

	// Set up pterm with a funky theme
	pterm.DefaultSection.Style = pterm.NewStyle(pterm.FgCyan, pterm.Bold)
	pterm.Info.Prefix = pterm.Prefix{Text: "INFO", Style: pterm.NewStyle(pterm.BgBlue, pterm.FgWhite)}
	pterm.Error.Prefix = pterm.Prefix{Text: "ERROR", Style: pterm.NewStyle(pterm.BgRed, pterm.FgWhite)}
	pterm.Success.Prefix = pterm.Prefix{Text: "SUCCESS", Style: pterm.NewStyle(pterm.BgGreen, pterm.FgBlack)}
	pterm.Warning.Prefix = pterm.Prefix{Text: "WARNING", Style: pterm.NewStyle(pterm.BgYellow, pterm.FgBlack)}

	if err := os.MkdirAll("logs", 0755); err != nil {
		pterm.Error.Println("Failed to create logs dir - universe says no:", err)
	}

	var err error
	dashboardTemplate, err = template.ParseFS(dashboardTemplateFS, "templates/dashboard.gohtml")
	if err != nil {
		pterm.Error.Println("Dashboard template parsing went kaput:", err)
	} else {
		//pterm.Success.Println("Dashboard template loaded - ready to rock!")
	}
}

func storeLog(message string) {
	logMutex.Lock()
	defer logMutex.Unlock()
	pid := os.Getpid()
	args := os.Args[1:]
	parameters := strings.Join(args, " ")

	logEntry := LogEntry{
		PID:        strconv.Itoa(pid),
		Parameters: parameters,
		Log:        message,
		Timestamp:  time.Now(),
	}
	inMemoryLogs = append(inMemoryLogs, logEntry)

	logFilePath := filepath.Join("logs", fmt.Sprintf("logs_%d.json", pid))
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		pterm.Error.Println("Log file opening failed - send help:", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(logEntry); err != nil {
		pterm.Error.Println("Log encoding broke - computers hate me:", err)
	}
}

func loadConfiguration(filePath string) *Configuration {
	//spinner, _ := pterm.DefaultSpinner.Start("Loading config - hold onto your hats!")
	if connect3270.Verbose {
		pterm.Info.Printf("Loading configuration from %s\n", filePath)
	}
	configFile, err := os.Open(filePath)
	if err != nil {
		pterm.Error.Printf("Error opening config file at %s: %v", filePath, err)
		os.Exit(1)
	}
	defer configFile.Close()
	var config Configuration
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		pterm.Error.Printf("Error decoding config JSON: %v", err)
	}
	if config.RampUpBatchSize <= 0 {
		config.RampUpBatchSize = 10
	}
	if config.RampUpDelay <= 0 {
		config.RampUpDelay = 1.0
	}
	err = validateConfiguration(&config)
	if err != nil {
		pterm.Error.Printf("Invalid configuration: %v", err)
	}
	//spinner.Success("Config loaded - we’re golden!")
	return &config
}

func loadInputFile(filePath string) ([]Step, error) {
	spinner, _ := pterm.DefaultSpinner.Start("Loading input file - fingers crossed!")
	if connect3270.Verbose {
		pterm.Info.Printf("Loading input file: %s\n", filePath)
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		spinner.Fail("Input file read failed - disk gremlins:", err)
		return nil, fmt.Errorf("error reading input file: %v", err)
	}
	if connect3270.Verbose {
		pterm.Success.Printf("Successfully read input file: %d bytes\n", len(data))
	}
	var steps []Step
	steps = append(steps, Step{Type: "Connect"})
	if connect3270.Verbose {
		pterm.Info.Println("Added initial Connect step")
	}
	lines := strings.Split(string(data), "\n")
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if connect3270.Verbose {
			pterm.Info.Printf("Processing line %d: %s", idx+1, line)
		}
		if strings.HasPrefix(line, "yield ps.sendKeys") {
			key := strings.TrimPrefix(line, "yield ps.sendKeys(")
			key = strings.TrimSuffix(key, ");")
			key = strings.Trim(key, "'")
			stepType := ""
			switch key {
			case "ControlKey.TAB":
				stepType = "PressTab"
			case "ControlKey.ENTER":
				stepType = "PressEnter"
			case "ControlKey.F1":
				stepType = "PressPF1"
			case "ControlKey.F2":
				stepType = "PressPF2"
			case "ControlKey.F3":
				stepType = "PressPF3"
			case "ControlKey.F4":
				stepType = "PressPF4"
			case "ControlKey.F5":
				stepType = "PressPF5"
			case "ControlKey.F6":
				stepType = "PressPF6"
			case "ControlKey.F7":
				stepType = "PressPF7"
			case "ControlKey.F8":
				stepType = "PressPF8"
			case "ControlKey.F9":
				stepType = "PressPF9"
			case "ControlKey.F10":
				stepType = "PressPF10"
			case "ControlKey.F11":
				stepType = "PressPF11"
			case "ControlKey.F12":
				stepType = "PressPF12"
			case "ControlKey.F13":
				stepType = "PressPF13"
			case "ControlKey.F14":
				stepType = "PressPF14"
			case "ControlKey.F15":
				stepType = "PressPF15"
			case "ControlKey.F16":
				stepType = "PressPF16"
			case "ControlKey.F17":
				stepType = "PressPF17"
			case "ControlKey.F18":
				stepType = "PressPF18"
			case "ControlKey.F19":
				stepType = "PressPF19"
			case "ControlKey.F20":
				stepType = "PressPF20"
			case "ControlKey.F21":
				stepType = "PressPF21"
			case "ControlKey.F22":
				stepType = "PressPF22"
			case "ControlKey.F23":
				stepType = "PressPF23"
			case "ControlKey.F24":
				stepType = "PressPF24"
			default:
				stepType = "FillString"
			}
			step := Step{Type: stepType, Text: key}
			steps = append(steps, step)
			if connect3270.Verbose {
				pterm.Info.Printf("Added step: %s with text: %s\n", stepType, key)
			}
		} else if strings.HasPrefix(line, "yield wait.forText") {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				text := strings.TrimPrefix(parts[0], "yield wait.forText('")
				text = strings.TrimSuffix(text, "'")
				position := strings.TrimPrefix(parts[1], "new Position(")
				position = strings.TrimSuffix(position, ");")
				posParts := strings.Split(position, ",")
				if len(posParts) == 2 {
					row, errRow := strconv.Atoi(strings.TrimSpace(posParts[0]))
					column, errCol := strconv.Atoi(strings.TrimSpace(posParts[1]))
					if errRow != nil || errCol != nil {
						if connect3270.Verbose {
							pterm.Warning.Printf("Error parsing position in line %d - numbers hate me\n", idx+1)
						}
						continue
					}
					step := Step{
						Type: "CheckValue",
						Coordinates: connect3270.Coordinates{
							Row:    row,
							Column: column,
							Length: len(text),
						},
						Text: text,
					}
					steps = append(steps, step)
					if connect3270.Verbose {
						pterm.Info.Printf("Added CheckValue step: text '%s' at (%d,%d), length %d\n", text, row, column, len(text))
					}
				}
			}
		} else if strings.HasPrefix(line, "// Fill in the first name at row") || strings.HasPrefix(line, "// Fill in the last name at row") {
			parts := strings.Split(line, " ")
			if len(parts) >= 8 {
				row, errRow := strconv.Atoi(parts[6])
				column, errCol := strconv.Atoi(parts[9])
				if errRow != nil || errCol != nil {
					if connect3270.Verbose {
						pterm.Warning.Printf("Error parsing coords in line %d - math is hard\n", idx+1)
					}
					continue
				}
				if idx+1 < len(lines) {
					nextLine := strings.TrimSpace(lines[idx+1])
					if strings.HasPrefix(nextLine, "yield ps.sendKeys") {
						key := strings.TrimPrefix(nextLine, "yield ps.sendKeys(")
						key = strings.TrimSuffix(key, ");")
						key = strings.Trim(key, "'")
						step := Step{
							Type: "FillString",
							Coordinates: connect3270.Coordinates{
								Row:    row,
								Column: column,
							},
							Text: key,
						}
						steps = append(steps, step)
						if connect3270.Verbose {
							pterm.Info.Printf("Added FillString step: text '%s' at (%d,%d)\n", key, row, column)
						}
					}
				}
			}
		}
	}
	steps = append(steps, Step{Type: "Disconnect"})
	if connect3270.Verbose {
		pterm.Info.Println("Added final Disconnect step")
		pterm.DefaultTable.WithHasHeader().WithData(pterm.TableData{
			{"Step", "Type", "Text", "Row", "Column", "Length"},
			{"", "", "", "", "", ""}, // Separator
		}).Render()
		for i, step := range steps {
			pterm.DefaultTable.WithData(pterm.TableData{
				{strconv.Itoa(i), step.Type, step.Text, strconv.Itoa(step.Coordinates.Row), strconv.Itoa(step.Coordinates.Column), strconv.Itoa(step.Coordinates.Length)},
			}).Render()
		}
	}
	spinner.Success("Input file loaded - we’re cooking with gas!")
	return steps, nil
}

func runWorkflow(scriptPort int, config *Configuration) error {
	startTime := time.Now()
	atomic.AddInt64(&totalWorkflowsStarted, 1)
	if connect3270.Verbose {
		pterm.Info.Printf("Starting workflow for scriptPort %d\n", scriptPort)
	}
	storeLog(fmt.Sprintf("Starting workflow for scriptPort %d", scriptPort))
	mutex.Lock()
	activeWorkflows++
	mutex.Unlock()
	e := connect3270.NewEmulator(config.Host, config.Port, strconv.Itoa(scriptPort))
	tmpFile, err := ioutil.TempFile("", "workflowOutput_")
	if err != nil {
		pterm.Error.Printf("Temp file creation failed - disk’s playing hide and seek: %v", err)
		return err
	}
	tmpFileName := tmpFile.Name()
	tmpFile.Close()
	e.InitializeOutput(tmpFileName, runAPI)
	workflowFailed := false
	var steps []Step
	if config.InputFilePath != "" {
		steps, err = loadInputFile(config.InputFilePath)
		if err != nil {
			pterm.Error.Printf("Input file load crashed - file has gone rogue: %v\n", err)
			return err
		}
	} else {
		steps = config.Steps
	}
	for _, step := range steps {
		if workflowFailed {
			break
		}
		switch step.Type {
		case "InitializeOutput":
			err := e.InitializeOutput(tmpFileName, runAPI)
			if err != nil {
				pterm.Error.Printf("Output init failed - setup’s cursed: %v\n", err)
				workflowFailed = true
			}
		case "Connect":
			if err := e.Connect(); err != nil {
				pterm.Error.Printf("Connection failed - terminal’s ghosting us: %v\n", err)
				workflowFailed = true
			}
			e.WaitForField(30)
		case "CheckValue":
			v, err := e.GetValue(step.Coordinates.Row, step.Coordinates.Column, step.Coordinates.Length)
			if err != nil {
				pterm.Error.Printf("Value grab failed - screen’s being shy: %v\n", err)
				workflowFailed = true
				break
			}
			v = strings.TrimSpace(v)
			if connect3270.Verbose {
				pterm.Info.Printf("Retrieved value: %s\n", v)
			}
			if v != step.Text {
				pterm.Warning.Printf("CheckValue mismatch - Expected: %s, Got: %s\n", step.Text, v)
				workflowFailed = true
			}
		case "FillString":
			if err := e.FillString(step.Coordinates.Row, step.Coordinates.Column, step.Text); err != nil {
				pterm.Error.Printf("Text fill failed - keyboard’s drunk: %v\n", err)
				workflowFailed = true
			}
		case "AsciiScreenGrab":
			if err := e.AsciiScreenGrab(tmpFileName, runAPI); err != nil {
				pterm.Error.Printf("Screen grab flopped - camera’s broken: %v\n", err)
				workflowFailed = true
			}
		case "PressEnter":
			if err := e.Press(connect3270.Enter); err != nil {
				pterm.Error.Printf("Enter press failed - button’s stuck: %v\n", err)
				workflowFailed = true
			}
		case "PressTab":
			if err := e.Press(connect3270.Tab); err != nil {
				pterm.Error.Printf("Tab press failed - spacing out: %v\n", err)
				workflowFailed = true
			}
		case "Disconnect":
			if err := e.Disconnect(); err != nil {
				pterm.Error.Printf("Disconnect failed - clinging on: %v\n", err)
				workflowFailed = true
			}
		case "PressPF1":
			if err := e.Press(connect3270.F1); err != nil {
				pterm.Error.Printf("PF1 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF2":
			if err := e.Press(connect3270.F2); err != nil {
				pterm.Error.Printf("PF2 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF3":
			if err := e.Press(connect3270.F3); err != nil {
				pterm.Error.Printf("PF3 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF4":
			if err := e.Press(connect3270.F4); err != nil {
				pterm.Error.Printf("PF4 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF5":
			if err := e.Press(connect3270.F5); err != nil {
				pterm.Error.Printf("PF5 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF6":
			if err := e.Press(connect3270.F6); err != nil {
				pterm.Error.Printf("PF6 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF7":
			if err := e.Press(connect3270.F7); err != nil {
				pterm.Error.Printf("PF7 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF8":
			if err := e.Press(connect3270.F8); err != nil {
				pterm.Error.Printf("PF8 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF9":
			if err := e.Press(connect3270.F9); err != nil {
				pterm.Error.Printf("PF9 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF10":
			if err := e.Press(connect3270.F10); err != nil {
				pterm.Error.Printf("PF10 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF11":
			if err := e.Press(connect3270.F11); err != nil {
				pterm.Error.Printf("PF11 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF12":
			if err := e.Press(connect3270.F12); err != nil {
				pterm.Error.Printf("PF12 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF13":
			if err := e.Press(connect3270.F13); err != nil {
				pterm.Error.Printf("PF13 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF14":
			if err := e.Press(connect3270.F14); err != nil {
				pterm.Error.Printf("PF14 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF15":
			if err := e.Press(connect3270.F15); err != nil {
				pterm.Error.Printf("PF15 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF16":
			if err := e.Press(connect3270.F16); err != nil {
				pterm.Error.Printf("PF16 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF17":
			if err := e.Press(connect3270.F17); err != nil {
				pterm.Error.Printf("PF17 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF18":
			if err := e.Press(connect3270.F18); err != nil {
				pterm.Error.Printf("PF18 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF19":
			if err := e.Press(connect3270.F19); err != nil {
				pterm.Error.Printf("PF19 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF20":
			if err := e.Press(connect3270.F20); err != nil {
				pterm.Error.Printf("PF20 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF21":
			if err := e.Press(connect3270.F21); err != nil {
				pterm.Error.Printf("PF21 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF22":
			if err := e.Press(connect3270.F22); err != nil {
				pterm.Error.Printf("PF22 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF23":
			if err := e.Press(connect3270.F23); err != nil {
				pterm.Error.Printf("PF23 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		case "PressPF24":
			if err := e.Press(connect3270.F24); err != nil {
				pterm.Error.Printf("PF24 press failed - function’s funky: %v\n", err)
				workflowFailed = true
			}
		default:
			pterm.Warning.Printf("Unknown step type: %s - what’s this sorcery?\n", step.Type)
		}
	}

	func runWorkflow(scriptPort int, config *Configuration) error {
    startTime := time.Now()
    atomic.AddInt64(&totalWorkflowsStarted, 1)
    if connect3270.Verbose {
        pterm.Info.Printf("Starting workflow for scriptPort %d\n", scriptPort)
    }
    storeLog(fmt.Sprintf("Starting workflow for scriptPort %d", scriptPort))
    mutex.Lock()
    activeWorkflows++
    mutex.Unlock()
    e := connect3270.NewEmulator(config.Host, config.Port, strconv.Itoa(scriptPort))
    tmpFile, err := ioutil.TempFile("", "workflowOutput_")
    if err != nil {
        return handleError(err, fmt.Sprintf("Temp file creation failed - disk’s playing hide and seek: %v", err))
    }
    tmpFileName := tmpFile.Name()
    tmpFile.Close()
    e.InitializeOutput(tmpFileName, runAPI)
    workflowFailed := false
    var steps []Step
    if config.InputFilePath != "" {
        steps, err = loadInputFile(config.InputFilePath)
        if err != nil {
            return handleError(err, fmt.Sprintf("Input file load crashed - file has gone rogue: %v\n", err))
        }
    } else {
        steps = config.Steps
    }

    for _, step := range steps {
        if workflowFailed {
            break
        }
        err := executeStep(e, step, tmpFileName)
        if err != nil {
            workflowFailed = true
            addError(err)
        }
    }

    mutex.Lock()
    activeWorkflows--
    mutex.Unlock()
    duration := time.Since(startTime).Seconds()
    timingsMutex.Lock()
    workflowDurations = append(workflowDurations, duration)
    timingsMutex.Unlock()

    if workflowFailed {
        atomic.AddInt64(&totalWorkflowsFailed, 1)
    } else {
        if connect3270.Verbose {
            storeLog(fmt.Sprintf("Workflow for scriptPort %d completed successfully", scriptPort))
        }
        if config.OutputFilePath != "" {
            _ = os.Remove(config.OutputFilePath)
            if err := os.Rename(tmpFileName, config.OutputFilePath); err != nil {
                pid := os.Getpid()
                uniqueOutputPath := fmt.Sprintf("%s.%d", config.OutputFilePath, pid)
                if err2 := os.Rename(tmpFileName, uniqueOutputPath); err2 != nil {
                    addError(err2)
                } else if verbose {
                    pterm.Info.Printf("Renamed to unique output file: %s\n", uniqueOutputPath)
                }
                return err
            }
        }
        atomic.AddInt64(&totalWorkflowsCompleted, 1)
    }
    return nil
}

func runAPIWorkflow() {
	if connect3270.Verbose {
		pterm.Info.Println("Starting API server mode - buckle up!")
	}
	connect3270.Headless = true
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.SetTrustedProxies(nil)
	r.POST("/api/execute", func(c *gin.Context) {
		var workflowConfig Configuration
		if err := c.ShouldBindJSON(&workflowConfig); err != nil {
			sendErrorResponse(c, http.StatusBadRequest, "Invalid request payload - JSON’s drunk", err)
			return
		}
		tmpFile, err := ioutil.TempFile("", "workflowOutput_")
		if err != nil {
			pterm.Error.Println("Temp file creation failed - disk’s napping:", err)
			sendErrorResponse(c, http.StatusInternalServerError, "Failed to create temp file", err)
			return
		}
		defer tmpFile.Close()
		tmpFileName := tmpFile.Name()
		scriptPort := getNextAvailablePort()
		e := connect3270.NewEmulator(workflowConfig.Host, workflowConfig.Port, strconv.Itoa(scriptPort))
		err = e.InitializeOutput(tmpFileName, true)
		if err != nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Output init failed - setup’s cursed", err)
			return
		}
		for _, step := range workflowConfig.Steps {
			if err := executeStep(e, step, tmpFileName); err != nil {
				sendErrorResponse(c, http.StatusInternalServerError, fmt.Sprintf("Step '%s' failed - oof", step.Type), err)
				e.Disconnect()
				return
			}
		}
		outputContents, err := e.ReadOutputFile(tmpFileName)
		if err != nil {
			sendErrorResponse(c, http.StatusInternalServerError, "Output read failed - file’s shy", err)
			return
		}
		e.Disconnect()
		c.JSON(http.StatusOK, gin.H{
			"returnCode": http.StatusOK,
			"status":     "okay",
			"message":    "Workflow executed successfully - high five!",
			"output":     outputContents,
		})
	})
	apiAddr := fmt.Sprintf(":%d", apiPort)
	pterm.Success.Printf("API server rocking on %s - let’s roll!\n", apiAddr)
	if err := r.Run(apiAddr); err != nil {
		pterm.Error.Printf("API server crashed - send coffee: %v\n", err)
	}
}

func executeStep(e *connect3270.Emulator, step Step, tmpFileName string) error {
	switch step.Type {
	case "InitializeOutput":
		return e.InitializeOutput(tmpFileName, runAPI)
	case "Connect":
		return e.Connect()
	case "CheckValue":
		_, err := e.GetValue(step.Coordinates.Row, step.Coordinates.Column, step.Coordinates.Length)
		return err
	case "FillString":
		if step.Coordinates.Row == 0 && step.Coordinates.Column == 0 {
			return e.SetString(step.Text)
		}
		return e.FillString(step.Coordinates.Row, step.Coordinates.Column, step.Text)
	case "AsciiScreenGrab":
		return e.AsciiScreenGrab(tmpFileName, runAPI)
	case "PressEnter":
		return e.Press(connect3270.Enter)
	case "PressTab":
		return e.Press(connect3270.Tab)
	case "Disconnect":
		return e.Disconnect()
	case "PressPF1":
		return e.Press(connect3270.F1)
	case "PressPF2":
		return e.Press(connect3270.F2)
	case "PressPF3":
		return e.Press(connect3270.F3)
	case "PressPF4":
		return e.Press(connect3270.F4)
	case "PressPF5":
		return e.Press(connect3270.F5)
	case "PressPF6":
		return e.Press(connect3270.F6)
	case "PressPF7":
		return e.Press(connect3270.F7)
	case "PressPF8":
		return e.Press(connect3270.F8)
	case "PressPF9":
		return e.Press(connect3270.F9)
	case "PressPF10":
		return e.Press(connect3270.F10)
	case "PressPF11":
		return e.Press(connect3270.F11)
	case "PressPF12":
		return e.Press(connect3270.F12)
	case "PressPF13":
		return e.Press(connect3270.F13)
	case "PressPF14":
		return e.Press(connect3270.F14)
	case "PressPF15":
		return e.Press(connect3270.F15)
	case "PressPF16":
		return e.Press(connect3270.F16)
	case "PressPF17":
		return e.Press(connect3270.F17)
	case "PressPF18":
		return e.Press(connect3270.F18)
	case "PressPF19":
		return e.Press(connect3270.F19)
	case "PressPF20":
		return e.Press(connect3270.F20)
	case "PressPF21":
		return e.Press(connect3270.F21)
	case "PressPF22":
		return e.Press(connect3270.F22)
	case "PressPF23":
		return e.Press(connect3270.F23)
	case "PressPF24":
		return e.Press(connect3270.F24)
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

func sendErrorResponse(c *gin.Context, statusCode int, message string, err error) {
	if connect3270.Verbose {
		pterm.Info.Println("Sending error response - oopsie daisy!")
	}
	c.JSON(statusCode, gin.H{
		"returnCode": statusCode,
		"status":     "error",
		"message":    message,
		"error":      err.Error(),
	})
}

func printBanner() {

	clear()

	pterm.DefaultBigText.
		WithLetters(
			putils.LettersFromStringWithStyle("3270", pterm.FgLightGreen.ToStyle()),
			putils.LettersFromStringWithStyle("Connect", pterm.FgWhite.ToStyle()),
		).
		Render()

	pterm.DefaultBasicText.Println("Version: " + pterm.LightGreen(version))
	pterm.DefaultBasicText.Println("Website: " + pterm.LightGreen("https://3270.io"))
	pterm.DefaultBasicText.Println("Author: " + pterm.LightGreen("EyUp"))

	pterm.Info.Println("Runtime Environment: " + pterm.LightYellow("./3270Connect ") + pterm.White(strings.Join(os.Args[1:], " ")))
	pterm.Println()
}

func main() {
    flag.Parse()
    printBanner()
    mutex.Lock()
    lastUsedPort = startPort
    mutex.Unlock()
    if *showVersion {
        pterm.Info.Printf("3270Connect Version: %s \n", version)
        os.Exit(0)
    }
    if showHelp {
        pterm.Info.Printf("3270Connect Version: %s - Here’s the manual!\n", version)
        flag.Usage()
        os.Exit(0)
    }
    setGlobalSettings()
    if concurrent > 1 || runtimeDuration > 0 {
        go runDashboard()
    }
    go monitorSystemUsage()
    if runApp != "" {
        switch runApp {
        case "1":
            app1.RunApplication(runAppPort)
            return
        case "2":
            app2.RunApplication(runAppPort)
            return
        default:
            pterm.Error.Printf("Invalid runApp value: %s - Did you mean 1 or 2?\n", runApp)
        }
    }
    config := loadConfiguration(configFile)
    if runAPI {
        runAPIWorkflow()
    } else {
        if concurrent > 1 {
            runConcurrentWorkflows(config)
        } else {
            runWorkflow(7000, config)
        }
        if concurrent > 1 && dashboardStarted {
            pterm.Info.Printf("All workflows completed but the dashboard is still running on port %d. Press Ctrl+C to exit.", dashboardPort)
            select {}
        }
    }
    showErrors()
}

func setGlobalSettings() {
	connect3270.Headless = headless
	connect3270.Verbose = verbose
}

func openDashboardEmbedded() {
	url := "http://localhost:9200/dashboard"
	if err := browser.OpenURL(url); err != nil {
		pterm.Error.Printf("Failed to open dashboard URL: %v\n", err)
	}
}

func runConcurrentWorkflows(config *Configuration) {
	overallStart := time.Now()
	semaphore := make(chan struct{}, concurrent)
	var wg sync.WaitGroup

	// Initialize MultiPrinter for all output
	multi := pterm.DefaultMultiPrinter

	// Define a fixed width for titles to align text
	const titleWidth = 30

	// Uniform progress bars
	durationBar, _ := pterm.DefaultProgressbar.
		WithTotal(runtimeDuration).
		WithTitle(pterm.Sprintf("%-*s", titleWidth, "  Run Duration  ")).
		WithWriter(multi.NewWriter()).
		WithBarCharacter("-").
		WithBarStyle(pterm.NewStyle(pterm.FgCyan)).
		WithShowPercentage(true).
		WithShowCount(false).
		WithShowElapsedTime(true).
		Start()

	activeBar, _ := pterm.DefaultProgressbar.
		WithTotal(concurrent).
		WithTitle(pterm.Sprintf("%-*s", titleWidth, "Active vUsers")).
		WithWriter(multi.NewWriter()).
		WithBarCharacter("█").
		WithBarStyle(pterm.NewStyle(pterm.FgCyan)).
		WithShowPercentage(true).
		WithShowCount(false).
		WithShowElapsedTime(true).
		Start()

	cpuBar, _ := pterm.DefaultProgressbar.
		WithTotal(100).
		WithTitle(pterm.Sprintf("%-*s", titleWidth, "CPU Usage")).
		WithWriter(multi.NewWriter()).
		WithBarCharacter("█").
		WithBarStyle(pterm.NewStyle(pterm.FgGreen)).
		WithShowPercentage(true).
		WithShowCount(false).
		WithShowElapsedTime(true).
		Start()

	memBar, _ := pterm.DefaultProgressbar.
		WithTotal(100).
		WithTitle(pterm.Sprintf("%-*s", titleWidth, "Memory Usage")).
		WithWriter(multi.NewWriter()).
		WithBarCharacter("█").
		WithBarStyle(pterm.NewStyle(pterm.FgGreen)).
		WithShowPercentage(true).
		WithShowCount(false).
		WithShowElapsedTime(true).
		Start()

	// Start the MultiPrinter
	multi.Start()

	// Group durationBar and activeBar in a table box
	//pterm.DefaultTable.
	//	WithHasHeader(false).
	//	WithBoxed(true).
	//	WithData(pterm.TableData{
	//		{durationBar.Title, activeBar.Title},
	//		{fmt.Sprintf("%d/%d", durationBar.Current, durationBar.Total), fmt.Sprintf("%d/%d", activeBar.Current, activeBar.Total)},
	//	}).
	//	Render()

	// Group cpuBar and memBar in a different table box
	//pterm.DefaultTable.
	//	WithHasHeader(false).
	//	WithBoxed(true).
	//	WithData(pterm.TableData{
	//		{cpuBar.Title, memBar.Title},
	//		{fmt.Sprintf("%d/%d", cpuBar.Current, cpuBar.Total), fmt.Sprintf("%d/%d", memBar.Current, memBar.Total)},
	//	}).
	//	Render()

	// Channel to stop the progress bar updates
	stopTicker := make(chan struct{})

	// Goroutine for real-time progress bar updates
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				elapsed := int(time.Since(overallStart).Seconds())
				if durationBar != nil {
					durationBar.Current = min(elapsed, runtimeDuration)
					if elapsed < runtimeDuration {
						durationBar.UpdateTitle(pterm.Sprintf("%-*s", titleWidth, fmt.Sprintf("Run Duration (%ds left)", runtimeDuration-elapsed)))
					} else {
						durationBar.UpdateTitle(pterm.Sprintf("%-*s", titleWidth, "Run Duration (Completed)"))
					}
				}
				if cpuBar != nil {
					cpuPercent, _ := cpu.Percent(0, false)
					if len(cpuPercent) > 0 {
						cpuBar.Current = int(cpuPercent[0])
					}
				}
				if memBar != nil {
					memStats, _ := mem.VirtualMemory()
					if memStats != nil {
						memBar.Current = int(memStats.UsedPercent)
					}
				}
				if activeBar != nil {
					activeBar.Current = len(semaphore)
					activeBar.UpdateTitle(pterm.Sprintf("%-*s", titleWidth, fmt.Sprintf("Active vUsers (%d/%d)", len(semaphore), concurrent)))
				}
				// Log status update
				cpuPercentLog, _ := cpu.Percent(0, false)
				memStatsLog, _ := mem.VirtualMemory()
				cpuVal := 0.0
				if len(cpuPercentLog) > 0 {
					cpuVal = cpuPercentLog[0]
				}
				memVal := 0.0
				if memStatsLog != nil {
					memVal = memStatsLog.UsedPercent
				}
				storeLog(fmt.Sprintf("Elapsed: %d, Active workflows: %d, CPU usage: %.2f%%, Memory usage: %.2f%%", elapsed, len(semaphore), cpuVal, memVal))
			case <-stopTicker:
				return
			}
		}
	}()

	// Scheduling workflows using nested loops
	for time.Since(overallStart) < time.Duration(runtimeDuration)*time.Second {
		for time.Since(overallStart) < time.Duration(runtimeDuration)*time.Second {
			freeSlots := concurrent - len(semaphore)
			if freeSlots <= 0 {
				time.Sleep(time.Duration(config.RampUpDelay * float64(time.Second)))
				break
			}
			batchSize := min(freeSlots, config.RampUpBatchSize)
			storeLog(fmt.Sprintf("Increasing batch by %d, current size is %d, new total target is %d",
				batchSize, len(semaphore), len(semaphore)+batchSize))
			for i := 0; i < batchSize; i++ {
				semaphore <- struct{}{}
				wg.Add(1)
				go func() {
					defer wg.Done()
					portToUse := getNextAvailablePort()
					err := runWorkflow(portToUse, config)
					if err != nil {
						if connect3270.Verbose {
							pterm.Error.Printf("Workflow on port %d error: %v\n", portToUse, err)
						}
						storeLog(fmt.Sprintf("Workflow on port %d error: %v", portToUse, err))
					}
					<-semaphore
				}()
			}
			cpuPercent, _ := cpu.Percent(0, false)
			memStats, _ := mem.VirtualMemory()
			storeLog(fmt.Sprintf("Currently active workflows: %d, CPU usage: %.2f%%, memory usage: %.2f%%",
				len(semaphore), cpuPercent[0], memStats.UsedPercent))
			time.Sleep(time.Duration(config.RampUpDelay * float64(time.Second)))
		}
		cpuPercent, _ := cpu.Percent(0, false)
		memStats, _ := mem.VirtualMemory()
		storeLog(fmt.Sprintf("Currently active workflows: %d, CPU usage: %.2f%%, memory usage: %.2f%%",
			len(semaphore), cpuPercent[0], memStats.UsedPercent))
		time.Sleep(time.Duration(config.RampUpDelay * float64(time.Second)))
	}

	multi.Stop()

	// Notify that no new workflows will be scheduled
	pterm.Info.Println("Run duration complete. Waiting for current workflows to finish...")
	wg.Wait()
	storeLog("All workflows completed after runtimeDuration ended.")

	// Stop the progress bar updates
	close(stopTicker)

	// Final update to duration bar
	elapsed := int(time.Since(overallStart).Seconds())
	durationBar.WithTotal(elapsed)
	durationBar.Current = elapsed
	durationBar.UpdateTitle(pterm.Sprintf("%-*s", titleWidth, fmt.Sprintf("Run Duration (%ds elapsed)", elapsed)))

	// Calculate averages for CPU and Memory usage
	var avgCPU, avgMem float64
	mutex.Lock()
	if len(cpuHistory) > 0 {
		var cpuSum float64
		for _, val := range cpuHistory {
			cpuSum += val
		}
		avgCPU = cpuSum / float64(len(cpuHistory))
	}
	if len(memHistory) > 0 {
		var memSum float64
		for _, val := range memHistory {
			memSum += val
		}
		avgMem = memSum / float64(len(memHistory))
	}
	mutex.Unlock()

	// Calculate average workflow completion time
	var avgWorkflowTime float64
	timingsMutex.Lock()
	if len(workflowDurations) > 0 {
		var totalDuration float64
		for _, d := range workflowDurations {
			totalDuration += d
		}
		avgWorkflowTime = totalDuration / float64(len(workflowDurations))
	}
	timingsMutex.Unlock()

	// Capture final stats
	finalActive := len(semaphore)
	finalStarted := atomic.LoadInt64(&totalWorkflowsStarted)
	finalCompleted := atomic.LoadInt64(&totalWorkflowsCompleted)
	finalFailed := atomic.LoadInt64(&totalWorkflowsFailed)

	clear()
	printBanner()

	pterm.Success.Println("All workflows wrapped up - Time for a victory lap!")

	// Display summary report
	pterm.DefaultSection.WithStyle(pterm.NewStyle(pterm.FgCyan)).Println("Run Summary - Performance Report")
	pterm.DefaultTable.
		WithHasHeader().
		WithBoxed(true).
		WithLeftAlignment().
		WithData(pterm.TableData{
			{"Metric", "Value", "Status"},
			{"Total Workflows Started", fmt.Sprintf("%d", finalStarted), "🚀 Launched"},
			{"Total Workflows Completed", fmt.Sprintf("%d", finalCompleted), "✅ Done"},
			{"Total Workflows Failed", fmt.Sprintf("%d", finalFailed), func() string {
				if finalFailed > 0 {
					return "💥 Oof"
				}
				return "🎉 Perfect"
			}()},
			{"Final Active vUsers", fmt.Sprintf("%d/%d", finalActive, concurrent), func() string {
				if finalActive > 0 {
					return "💥 Oof"
				}
				return "🎉 Perfect"
			}()},
			{"Average CPU Usage", fmt.Sprintf("%.1f%%", avgCPU), cpuStatus(avgCPU)},
			{"Average Memory Usage", fmt.Sprintf("%.1f%%", avgMem), memStatus(avgMem)},
			{"Average Workflow Time", fmt.Sprintf("%.2fs", avgWorkflowTime), "⏱️ Avg Duration"},
			{"Run Duration", fmt.Sprintf("%ds", elapsed), "⏱️ Completed"},
		}).Render()

	// Note: If you already print the dashboard message in main, you might remove this duplicate.
	storeLog("All workflows completed or timed out")
}

// Helper functions for summary status
func cpuStatus(cpu float64) string {
	switch {
	case cpu < 50:
		return "🟢 Optimal"
	case cpu < 80:
		return "🟡 Moderate"
	default:
		return "🔴 High"
	}
}

func memStatus(mem float64) string {
	switch {
	case mem < 50:
		return "🟢 Optimal"
	case mem < 80:
		return "🟡 Moderate"
	default:
		return "🔴 High"
	}
}

func clear() {
	print("\033[H\033[2J")
}

func getNextAvailablePort() int {
	mutex.Lock()
	defer mutex.Unlock()
	for {
		lastUsedPort++
		if isPortAvailable(lastUsedPort) {
			return lastUsedPort
		}
		if connect3270.Verbose {
			pterm.Warning.Printf("Port %d is taken - port party’s full!\n", lastUsedPort)
		}
	}
}

func isPortAvailable(port int) bool {
	addr := ":" + strconv.Itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if connect3270.Verbose {
			pterm.Info.Printf("Port %d in use - next contestant please!\n", port)
		}
		return false
	}
	ln.Close()
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func validateConfiguration(config *Configuration) error {
	if connect3270.Verbose {
		pterm.Info.Println("Validating config - let’s see if it’s naughty or nice!")
	}
	if config.Host == "" {
		return fmt.Errorf("host is empty - where’s the party at?")
	}
	if config.Port <= 0 {
		return fmt.Errorf("port is invalid - ports cant be negative silly")
	}
	if config.OutputFilePath == "" {
		hasScreenGrab := false
		for _, step := range config.Steps {
			if step.Type == "AsciiScreenGrab" {
				hasScreenGrab = true
				break
			}
		}
		if hasScreenGrab {
			return fmt.Errorf("output file path is empty - screen grab needs a home")
		}
	}
	for _, step := range config.Steps {
		switch step.Type {
		case "Connect", "AsciiScreenGrab", "PressEnter", "Disconnect":
			continue
		case "CheckValue", "FillString":
			if step.Coordinates.Row == 0 || step.Coordinates.Column == 0 {
				return fmt.Errorf("coords missing in %s step - lost in space", step.Type)
			}
			if step.Text == "" {
				return fmt.Errorf("text empty in %s step - cat got your tongue?", step.Type)
			}
		default:
			return fmt.Errorf("unknown step type: %s - what’s this nonsense?", step.Type)
		}
	}
	return nil
}

func runDashboard() {
	addr := fmt.Sprintf(":%d", dashboardPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		pterm.Warning.Printf("Dashboard already vibing on port %d - skipping the encore!\n", dashboardPort)
		go func() {
			for {
				updateMetricsFile()
				time.Sleep(2 * time.Second)
			}
		}()
		return
	}
	dashboardStarted = true
	//openDashboardEmbedded()
	spinner, _ := pterm.DefaultSpinner.WithRemoveWhenDone(true).Start("Cleaning up old metrics - sweeping the floor!")
	dashboardDir, err := os.UserConfigDir()
	if err != nil {
		pterm.Warning.Println("Can’t find config dir - defaulting to local:", err)
		dashboardDir = filepath.Join(".", "dashboard")
	} else {
		dashboardDir = filepath.Join(dashboardDir, "3270Connect", "dashboard")
	}
	files, err := filepath.Glob(filepath.Join(dashboardDir, "metrics_*.json"))
	if err != nil {
		spinner.Warning("Error listing old metrics - file system’s trolling:", err)
	} else {
		for _, f := range files {
			if err := os.Remove(f); err != nil {
				pterm.Warning.Printf("Failed to yeet old metrics file %s: %v\n", f, err)
			} else {
				//pterm.Info.Printf("Old metrics file %s gone - poof!\n", f)
			}
		}
	}
	logFiles, err := filepath.Glob(filepath.Join("logs", "logs_*.json"))
	if err == nil {
		for _, lf := range logFiles {
			if err := os.Remove(lf); err != nil {
				//pterm.Warning.Printf("Failed to nuke old log file %s: %v\n", lf, err)
			} else {
				//pterm.Info.Printf("Old log file %s vaporized!\n", lf)
			}
		}
	}
	spinner.Success("Cleanup done - dashboard’s fresh as a daisy!")

	setupConsoleHandler()
	setupTerminalConsoleHandler()
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		files, err := filepath.Glob(filepath.Join(dashboardDir, "metrics_*.json"))
		if err != nil {
			pterm.Warning.Println("Error listing metrics files:", err)
			files = []string{}
		}
		var metricsList []Metrics
		var totalStarted, totalCompleted, totalFailed, active int
		for _, f := range files {
			data, err := ioutil.ReadFile(f)
			if err != nil {
				pterm.Warning.Printf("Error reading metrics file %s: %v\n", f, err)
				continue
			}
			var m Metrics
			if err := json.Unmarshal(data, &m); err != nil {
				pterm.Warning.Printf("Error unmarshaling metrics %s: %v\n", f, err)
				continue
			}
			metricsList = append(metricsList, m)
			totalStarted += int(m.TotalWorkflowsStarted)
			totalCompleted += int(m.TotalWorkflowsCompleted)
			totalFailed += int(m.TotalWorkflowsFailed)
			active += m.ActiveWorkflows
		}
		var hostMetrics *Metrics
		if len(metricsList) > 0 {
			hostMetrics = &metricsList[0]
			for i := 1; i < len(metricsList); i++ {
				if metricsList[i].PID < hostMetrics.PID {
					hostMetrics = &metricsList[i]
				}
			}
		}
		metricsJSON, _ := json.Marshal(metricsList)
		autoRefresh := r.URL.Query().Get("autoRefresh")
		refreshPeriod := r.URL.Query().Get("refreshPeriod")
		if refreshPeriod == "" {
			refreshPeriod = "5"
		}
		checked := ""
		if autoRefresh == "true" {
			checked = "checked"
		}
		sel1, sel5, sel10, sel15, sel30 := "", "", "", "", ""
		switch refreshPeriod {
		case "1":
			sel1 = "selected"
		case "5":
			sel5 = "selected"
		case "10":
			sel10 = "selected"
		case "15":
			sel15 = "selected"
		case "30":
			sel30 = "selected"
		}
		agg := aggregateMetrics()
		data := struct {
			ActiveWorkflows                 int
			TotalWorkflowsStarted           int64
			TotalWorkflowsCompleted         int64
			TotalWorkflowsFailed            int64
			Checked                         string
			Sel1, Sel5, Sel10, Sel15, Sel30 string
			Year                            int
			AutoRefreshEnabled              bool
			RefreshPeriod                   string
			MetricsJSON                     string
		}{
			ActiveWorkflows:         agg.ActiveWorkflows,
			TotalWorkflowsStarted:   agg.TotalWorkflowsStarted,
			TotalWorkflowsCompleted: agg.TotalWorkflowsCompleted,
			TotalWorkflowsFailed:    agg.TotalWorkflowsFailed,
			Checked:                 checked,
			Sel1:                    sel1,
			Sel5:                    sel5,
			Sel10:                   sel10,
			Sel15:                   sel15,
			Sel30:                   sel30,
			Year:                    time.Now().Year(),
			AutoRefreshEnabled:      autoRefresh == "true",
			RefreshPeriod:           refreshPeriod,
			MetricsJSON:             string(metricsJSON),
		}
		if err := dashboardTemplate.Execute(w, data); err != nil {
			pterm.Error.Printf("Dashboard template execution failed - HTML’s throwing a tantrum: %v\n", err)
		}
	})
	pterm.Info.Printf("Dashboard live at %s - check it out!\n", pterm.FgBlue.Sprintf("http://localhost:%d/dashboard", dashboardPort))
	pterm.Println()
	go func() {
		for {
			updateMetricsFile()
			time.Sleep(2 * time.Second)
		}
	}()
	if err := http.Serve(listener, nil); err != nil {
		pterm.Error.Printf("Dashboard server crashed - send a medic: %v\n", err)
	}
}

type Metrics struct {
	PID                     int       `json:"pid"`
	ActiveWorkflows         int       `json:"activeWorkflows"`
	TotalWorkflowsStarted   int64     `json:"totalWorkflowsStarted"`
	TotalWorkflowsCompleted int64     `json:"totalWorkflowsCompleted"`
	TotalWorkflowsFailed    int64     `json:"totalWorkflowsFailed"`
	Durations               []float64 `json:"durations"`
	CPUUsage                []float64 `json:"cpuUsage"`
	MemoryUsage             []float64 `json:"memoryUsage"`
	Params                  string    `json:"params"`
}

func updateMetricsFile() {
	cpuPercents, err := cpu.Percent(0, false)
	var hostCPU float64 = 0
	if err == nil && len(cpuPercents) > 0 {
		hostCPU = cpuPercents[0]
	}
	memStats, err := mem.VirtualMemory()
	var hostMem float64 = 0
	if err == nil {
		hostMem = memStats.UsedPercent
	}
	mutex.Lock()
	cpuHistory = append(cpuHistory, hostCPU)
	memHistory = append(memHistory, hostMem)
	mutex.Unlock()
	timingsMutex.Lock()
	durationsCopy := make([]float64, len(workflowDurations))
	copy(durationsCopy, workflowDurations)
	timingsMutex.Unlock()
	pid := os.Getpid()
	args := os.Args[1:]
	parameters := strings.Join(args, " ")
	metrics := Metrics{
		PID:                     pid,
		ActiveWorkflows:         getActiveWorkflows(),
		TotalWorkflowsStarted:   atomic.LoadInt64(&totalWorkflowsStarted),
		TotalWorkflowsCompleted: atomic.LoadInt64(&totalWorkflowsCompleted),
		TotalWorkflowsFailed:    atomic.LoadInt64(&totalWorkflowsFailed),
		Durations:               durationsCopy,
		CPUUsage:                cpuHistory,
		MemoryUsage:             memHistory,
		Params:                  parameters,
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		pterm.Warning.Printf("Metrics marshaling failed for pid %d - JSON’s sulking: %v\n", pid, err)
		return
	}
	dashboardDir, err := os.UserConfigDir()
	if err != nil {
		pterm.Warning.Println("User config dir fetch failed - going local:", err)
		dashboardDir = filepath.Join(".", "dashboard")
	} else {
		dashboardDir = filepath.Join(dashboardDir, "3270Connect", "dashboard")
	}
	os.MkdirAll(dashboardDir, 0755)
	filePath := filepath.Join(dashboardDir, fmt.Sprintf("metrics_%d.json", pid))
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		pterm.Warning.Printf("Metrics file write failed for pid %d - disk’s grumpy: %v\n", pid, err)
	}
}

func aggregateMetrics() Metrics {
	dashboardDir, err := os.UserConfigDir()
	if err != nil {
		pterm.Warning.Println("User config dir fetch failed:", err)
		dashboardDir = filepath.Join(".", "dashboard")
	} else {
		dashboardDir = filepath.Join(dashboardDir, "3270Connect", "dashboard")
	}
	files, err := filepath.Glob(filepath.Join(dashboardDir, "metrics_*.json"))
	if err != nil {
		pterm.Warning.Println("Metrics files listing failed:", err)
		return Metrics{}
	}
	var agg Metrics
	for _, f := range files {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			pterm.Warning.Printf("Reading file %s failed: %v\n", f, err)
			continue
		}
		var m Metrics
		if err := json.Unmarshal(data, &m); err != nil {
			pterm.Warning.Printf("Unmarshaling file %s failed: %v\n", f, err)
			continue
		}
		agg.TotalWorkflowsStarted += m.TotalWorkflowsStarted
		agg.TotalWorkflowsCompleted += m.TotalWorkflowsCompleted
		agg.TotalWorkflowsFailed += m.TotalWorkflowsFailed
		agg.ActiveWorkflows += m.ActiveWorkflows
		agg.Durations = append(agg.Durations, m.Durations...)
		agg.CPUUsage = append(agg.CPUUsage, m.CPUUsage...)
		agg.MemoryUsage = append(agg.MemoryUsage, m.MemoryUsage...)
	}
	return agg
}

func monitorSystemUsage() {
	for {
		cpuPercents, err := cpu.Percent(1*time.Second, true)
		if err == nil && len(cpuPercents) > 0 {
			var sum float64
			for _, p := range cpuPercents {
				sum += p
			}
			overall := sum / float64(len(cpuPercents))
			mutex.Lock()
			cpuHistory = append(cpuHistory, overall)
			if len(cpuHistory) > 100 {
				cpuHistory = cpuHistory[1:]
			}
			mutex.Unlock()
		}
		memStats, err := mem.VirtualMemory()
		if err == nil {
			mutex.Lock()
			memHistory = append(memHistory, memStats.UsedPercent)
			if len(memHistory) > 100 {
				memHistory = memHistory[1:]
			}
			mutex.Unlock()
		}
	}
}

func setupConsoleHandler() {
	http.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		pidFilter := r.URL.Query().Get("pid")
		var filtered []LogEntry
		if pidFilter != "" {
			logFilePath := filepath.Join("logs", fmt.Sprintf("logs_%s.json", pidFilter))
			file, err := os.Open(logFilePath)
			if err != nil {
				pterm.Warning.Printf("Log file opening failed for PID %s: %v\n", pidFilter, err)
				http.Error(w, "Error opening log file", http.StatusInternalServerError)
				return
			}
			defer file.Close()
			decoder := json.NewDecoder(file)
			for {
				var logEntry LogEntry
				if err := decoder.Decode(&logEntry); err != nil {
					if err == io.EOF {
						break
					}
					pterm.Warning.Println("Log entry decoding failed:", err)
					http.Error(w, "Error decoding log entry", http.StatusInternalServerError)
					return
				}
				filtered = append(filtered, logEntry)
			}
		} else {
			logFiles, err := filepath.Glob(filepath.Join("logs", "logs_*.json"))
			if err == nil {
				for _, lf := range logFiles {
					file, err := os.Open(lf)
					if err != nil {
						pterm.Warning.Printf("Log file %s opening failed: %v\n", lf, err)
						continue
					}
					defer file.Close()
					decoder := json.NewDecoder(file)
					for {
						var logEntry LogEntry
						if err := decoder.Decode(&logEntry); err != nil {
							if err == io.EOF {
								break
							}
							pterm.Warning.Println("Log entry decoding failed:", err)
							continue
						}
						filtered = append(filtered, logEntry)
					}
				}
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Timestamp.After(filtered[j].Timestamp)
		})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(filtered)
	})
}

func setupTerminalConsoleHandler() {
	http.HandleFunc("/terminal-console", func(w http.ResponseWriter, r *http.Request) {
		pidFilter := r.URL.Query().Get("pid")
		var filtered []LogEntry
		if pidFilter != "" {
			logFilePath := filepath.Join("logs", fmt.Sprintf("logs_%s.json", pidFilter))
			file, err := os.Open(logFilePath)
			if err != nil {
				pterm.Warning.Printf("Log file opening failed for PID %s: %v\n", pidFilter, err)
				http.Error(w, "Error opening log file", http.StatusInternalServerError)
				return
			}
			defer file.Close()
			decoder := json.NewDecoder(file)
			for {
				var logEntry LogEntry
				if err := decoder.Decode(&logEntry); err != nil {
					if err == io.EOF {
						break
					}
					pterm.Warning.Println("Log entry decoding failed:", err)
					http.Error(w, "Error decoding log entry", http.StatusInternalServerError)
					return
				}
				filtered = append(filtered, logEntry)
			}
		} else {
			logFiles, err := filepath.Glob(filepath.Join("logs", "logs_*.json"))
			if err == nil {
				for _, lf := range logFiles {
					file, err := os.Open(lf)
					if err != nil {
						pterm.Warning.Printf("Log file %s opening failed: %v\n", lf, err)
						continue
					}
					defer file.Close()
					decoder := json.NewDecoder(file)
					for {
						var logEntry LogEntry
						if err := decoder.Decode(&logEntry); err != nil {
							if err == io.EOF {
								break
							}
							pterm.Warning.Println("Log entry decoding failed:", err)
							continue
						}
						filtered = append(filtered, logEntry)
					}
				}
			}
		}
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Timestamp.After(filtered[j].Timestamp)
		})
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		for _, entry := range filtered {
			w.Write([]byte(fmt.Sprintf("%s\n", entry.Log)))
		}
	})
}

func getActiveWorkflows() int {
	if connect3270.Verbose {
		pterm.Info.Println("Counting active workflows - herd the cats!")
	}
	mutex.Lock()
	defer mutex.Unlock()
	return activeWorkflows
}
