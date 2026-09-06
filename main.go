package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/natefinch/lumberjack"
)

var (
	cyan      = lipgloss.Color("#00f5d4")
	gray      = lipgloss.Color("#4a5568")
	lightGray = lipgloss.Color("#a0aec0")
	white     = lipgloss.Color("#ffffff")
	red       = lipgloss.Color("#ff0054")
	green     = lipgloss.Color("#7bf1a8")
	yellow    = lipgloss.Color("#ffee32")
	blue      = lipgloss.Color("#00bbf9")
	darkBg    = lipgloss.Color("#1a202c")

	panelTitleStyle  = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	activeTitleStyle = lipgloss.NewStyle().Foreground(cyan).Bold(true).Underline(true)
	labelStyle       = lipgloss.NewStyle().Foreground(gray).Bold(true)
	valueStyle       = lipgloss.NewStyle().Foreground(white)
	highlightStyle   = lipgloss.NewStyle().Foreground(green).Bold(true)

	keyStyle  = lipgloss.NewStyle().Background(lightGray).Foreground(darkBg).Bold(true).Padding(0, 1)
	descStyle = lipgloss.NewStyle().Foreground(lightGray).Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
	Border(lipgloss.NormalBorder()).
	BorderForeground(gray).
	Padding(0, 1)

	activeBoxStyle = boxStyle.Copy().
	BorderForeground(cyan)

	errorBoxStyle = boxStyle.Copy().
	BorderForeground(red)

	selectedRowStyle = lipgloss.NewStyle().Background(lipgloss.Color("#2d3748")).Foreground(white).Bold(true)
)

type activePanel int

const (
	searchPanel activePanel = iota
	forecastPanel
)

type errMsg error

type model struct {
	textInput     textinput.Model
	spinner       spinner.Model
	client        *WeatherClient
	weather       WeatherResponse
	cityName      string
	lastQuery     string
	err           error
	loading       bool
	hasData       bool
	useFahrenheit bool
	activePanel   activePanel
	selectedRow   int
	termWidth     int
	termHeight    int
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "Enter city identifier..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 32

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(cyan)

	return model{
		textInput:     ti,
		spinner:       s,
		client:        NewWeatherClient(),
		activePanel:   searchPanel,
		useFahrenheit: false,
		selectedRow:   0,
		termWidth:     100,
		termHeight:    24,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

type weatherMsg struct {
	data string
	w    WeatherResponse
}

func celsiusToFahrenheit(c float64) float64 {
	return (c * 9 / 5) + 32
}

func (m model) fetchWeatherCmd(city string, forceRefresh bool) tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			w, name, err := m.client.GetWeather(city, forceRefresh)
			if err != nil {
				return errMsg(err)
			}
			return weatherMsg{data: name, w: w}
		},
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil

		case tea.KeyMsg:
			switch msg.Type {
				case tea.KeyCtrlC, tea.KeyEsc:
					return m, tea.Quit

				case tea.KeyTab:
					if m.activePanel == searchPanel {
						m.activePanel = forecastPanel
						m.textInput.Blur()
					} else {
						m.activePanel = searchPanel
						m.textInput.Focus()
					}
					return m, nil

				case tea.KeyEnter:
					if m.activePanel == searchPanel {
						city := strings.TrimSpace(m.textInput.Value())
						if city == "" {
							return m, nil
						}
						m.loading = true
						m.err = nil
						m.lastQuery = city
						return m, m.fetchWeatherCmd(city, false)
					}

				case tea.KeyUp, tea.KeyDown, tea.KeyRunes:
					key := msg.String()

					// Scope navigation and shortcuts to specific panels to prevent interference during search input typing.
					if m.activePanel == forecastPanel {
						if m.hasData {
							switch key {
								case "k", "up":
									if m.selectedRow > 0 {
										m.selectedRow--
									}
									return m, nil
								case "j", "down":
									if m.selectedRow < len(m.weather.Daily)-1 {
										m.selectedRow++
									}
									return m, nil
							}
						}

						if key == "u" || key == "U" {
							m.useFahrenheit = !m.useFahrenheit
							return m, nil
						}

						if (key == "r" || key == "R") && m.lastQuery != "" {
							m.loading = true
							m.err = nil
							return m, m.fetchWeatherCmd(m.lastQuery, true)
						}
					}
			}

								case weatherMsg:
									m.loading = false
									m.hasData = true
									m.cityName = msg.data
									m.weather = msg.w
									m.selectedRow = 0
									m.textInput.SetValue("")
									return m, nil

								case errMsg:
									m.loading = false
									m.hasData = false
									m.err = msg
									return m, nil

								case spinner.TickMsg:
									var cmd tea.Cmd
									m.spinner, cmd = m.spinner.Update(msg)
									if m.loading {
										cmds = append(cmds, cmd)
									}
	}

	if m.activePanel == searchPanel {
		var inputCmd tea.Cmd
		m.textInput, inputCmd = m.textInput.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	availableHeight := m.termHeight - 4
	if availableHeight < 14 {
		availableHeight = 14
	}

	leftWidth := int(float64(m.termWidth) * 0.35)
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.termWidth - leftWidth - 2
	if rightWidth < 45 {
		rightWidth = 45
	}

	const searchBoxHeight = 5
	currentBoxHeight := availableHeight - searchBoxHeight

	searchInnerHeight := searchBoxHeight - 2
	currentInnerHeight := currentBoxHeight - 2
	forecastInnerHeight := availableHeight - 2

		// ------------------------------------------------------------------------
		// 1. SEARCH PANEL
		// ------------------------------------------------------------------------
		searchTitle := " COMPONENT: SEARCH ENGINE "
		searchBoxStyleToUse := boxStyle
		if m.activePanel == searchPanel {
			searchBoxStyleToUse = activeBoxStyle
		}

		searchContent := fmt.Sprintf("%s\n\n%s", panelTitleStyle.Render(searchTitle), m.textInput.View())
		searchBox := searchBoxStyleToUse.Width(leftWidth - 2).Height(searchInnerHeight).Render(searchContent)

		// ------------------------------------------------------------------------
		// 2. ATMOSPHERE MONITOR PANEL
		// ------------------------------------------------------------------------
		var currentBox string

		currentHeaderTitle := " MONITOR: ATMOSPHERE "
		if m.hasData && m.cityName != "" {
			currentHeaderTitle = fmt.Sprintf(" MONITOR: %s ", m.cityName)
		}
		currentHeader := panelTitleStyle.Render(currentHeaderTitle)

		currentStyle := boxStyle
		if m.err != nil {
			currentStyle = errorBoxStyle
		}

		if m.loading {
			loadingText := fmt.Sprintf("%s Fetching telemetry pipeline...", m.spinner.View())
			placeholderText := fmt.Sprintf("STATUS: PARSING METRICS...\n\n%s\nSynchronizing Open-Meteo DB\nStream pipelines active...", loadingText)
			currentBox = currentStyle.Width(leftWidth - 2).Height(currentInnerHeight).Render(currentHeader + "\n\n" + placeholderText)
		} else if m.hasData {
			aqiVal := m.weather.Current.AirQualityIndex
			var aqiDesc string
			if aqiVal <= 20 {
				aqiDesc = lipgloss.NewStyle().Foreground(green).Render(fmt.Sprintf("%d (EXCELLENT)", aqiVal))
			} else if aqiVal <= 40 {
				aqiDesc = lipgloss.NewStyle().Foreground(yellow).Render(fmt.Sprintf("%d (POOR)", aqiVal))
			} else {
				aqiDesc = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("%d (CRITICAL)", aqiVal))
			}

			tempVal := m.weather.Current.Temperature
			unitStr := "°C"
			if m.useFahrenheit {
				tempVal = celsiusToFahrenheit(tempVal)
				unitStr = "°F"
			}

			cacheBadge := ""
			if m.weather.FromCache {
				cacheBadge = lipgloss.NewStyle().Foreground(yellow).Bold(true).Render(" [CACHE HIT]")
			}

			metricsContent := fmt.Sprintf(
				"%s  %-12s %s%s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s",
				 labelStyle.Render("[TEMP]"), "Temperature:", highlightStyle.Render(fmt.Sprintf("%.1f %s", tempVal, unitStr)), cacheBadge,
						      labelStyle.Render("[HUMI]"), "Humidity:", valueStyle.Render(fmt.Sprintf("%d%%", m.weather.Current.Humidity)),
						      labelStyle.Render("[WIND]"), "Wind Speed:", valueStyle.Render(fmt.Sprintf("%.1f km/h", m.weather.Current.WindSpeed)),
						      labelStyle.Render("[SUNR]"), "Sun Rise:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunrise),
						      labelStyle.Render("[SUNS]"), "Sun Set:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunset),
						      labelStyle.Render("[AQI ]"), "Air Quality:", aqiDesc,
			)

			fullCurrentView := currentHeader + "\n\n" + lipgloss.PlaceVertical(currentInnerHeight-3, lipgloss.Top, metricsContent)
			currentBox = currentStyle.Width(leftWidth - 2).Height(currentInnerHeight).Render(fullCurrentView)
		} else {
			placeholderText := "STATUS: SYSTEM IDLE\n\nAwaiting dispatcher query...\nInsert location name above."
			if m.err != nil {
				placeholderText = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.err))
			}
			currentBox = currentStyle.Width(leftWidth - 2).Height(currentInnerHeight).Render(currentHeader + "\n\n" + placeholderText)
		}

		leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, currentBox)

		// ------------------------------------------------------------------------
		// 3. 14-DAY FORECAST PANEL
		// ------------------------------------------------------------------------
		var forecastBox string
		forecastHeader := panelTitleStyle.Render(" METRIC: 14-DAY CORE FORECAST ") + "\n\n"

			forecastBoxStyleToUse := boxStyle
				if m.activePanel == forecastPanel {
					forecastBoxStyleToUse = activeBoxStyle
				}

				const fixedTableColumnsWidth = 36
				dynamicBarLength := rightWidth - fixedTableColumnsWidth - 9
				if dynamicBarLength < 10 {
					dynamicBarLength = 10
				}

				graphHeaderPadding := strings.Repeat(" ", maxInt(0, dynamicBarLength-15))
				unitHeader := "MAX (°C)│ MIN (°C)"
				if m.useFahrenheit {
					unitHeader = "MAX (°F)│ MIN (°F)"
				}
				tableHeader := lipgloss.NewStyle().Foreground(lightGray).Render(fmt.Sprintf(" DATE       │ %s │ PRECIPITATION GRAPH%s", unitHeader, graphHeaderPadding)) + "\n"
				tableDivider := lipgloss.NewStyle().Foreground(gray).Render(strings.Repeat("─", maxInt(10, rightWidth-4))) + "\n"

				var tableRows strings.Builder
				if m.hasData {
					for i, day := range m.weather.Daily {
						filledBlocks := (day.PrecipProbability * dynamicBarLength) / 100

						var barStr strings.Builder
						barStr.WriteString("[")
						for b := 0; b < dynamicBarLength; b++ {
							if b < filledBlocks {
								barStr.WriteString("█")
							} else {
								barStr.WriteString("░")
							}
						}
						barStr.WriteString("]")

						var barStyle lipgloss.Style
						if day.PrecipProbability > 70 {
							barStyle = lipgloss.NewStyle().Foreground(red)
						} else if day.PrecipProbability > 30 {
							barStyle = lipgloss.NewStyle().Foreground(yellow)
						} else {
							barStyle = lipgloss.NewStyle().Foreground(gray)
						}

						renderedBar := barStyle.Render(fmt.Sprintf("%s %3d%%", barStr.String(), day.PrecipProbability))

						maxTemp := day.MaxTemp
						minTemp := day.MinTemp
						if m.useFahrenheit {
							maxTemp = celsiusToFahrenheit(maxTemp)
							minTemp = celsiusToFahrenheit(minTemp)
						}

						row := fmt.Sprintf(" %-10s │  %-7.1f │  %-7.1f │ %s", day.Date, maxTemp, minTemp, renderedBar)

						if m.activePanel == forecastPanel && i == m.selectedRow {
							row = selectedRowStyle.Render(row)
						}

						tableRows.WriteString(row + "\n")
					}

					contentLines := strings.Split(tableRows.String(), "\n")
					maxAllowedRows := forecastInnerHeight - 4
					if len(contentLines) > maxAllowedRows && maxAllowedRows > 0 {
						contentLines = contentLines[:maxAllowedRows]
					}

					forecastBox = forecastBoxStyleToUse.Width(rightWidth).Height(forecastInnerHeight).Render(forecastHeader + tableHeader + tableDivider + strings.Join(contentLines, "\n"))
				} else {
					statusMsg := "[WAIT] Pipeline awaiting telemetry input..."
					if m.loading {
						statusMsg = fmt.Sprintf("%s Streaming telemetry from Open-Meteo clusters...", m.spinner.View())
					}
					emptyLines := fmt.Sprintf("\n\n\n\n\n\n\n          %s", statusMsg)
					forecastBox = forecastBoxStyleToUse.Width(rightWidth).Height(forecastInnerHeight).Render(forecastHeader + tableHeader + tableDivider + emptyLines)
				}

				mainDashboard := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, forecastBox)

				// ------------------------------------------------------------------------
				// 4. FOOTER STATUS BAR
				// ------------------------------------------------------------------------
				navKeys := keyStyle.Render("Tab") + descStyle.Render("Switch View")

				if m.activePanel == forecastPanel {
					navKeys += keyStyle.Render("u") + descStyle.Render("Toggle °C/°F") +
					keyStyle.Render("r") + descStyle.Render("Refresh Cache") +
					keyStyle.Render("j/k") + descStyle.Render("Navigate Rows")
				}

				statusBar := lipgloss.JoinHorizontal(lipgloss.Left,
								     keyStyle.Render("Enter"), descStyle.Render("Search"),
								     navKeys,
					 keyStyle.Render("Esc"), descStyle.Render("Exit"),
								     lipgloss.NewStyle().Foreground(gray).Padding(0, 1).Render("│"),
								     lipgloss.NewStyle().Foreground(cyan).Italic(true).Render(fmt.Sprintf("WeatherTUI - [res: %dx%d]", m.termWidth, m.termHeight)),
				)

				return "\n" + mainDashboard + "\n\n" + statusBar + "\n"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getLogFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	logDir := filepath.Join(homeDir, ".local", "share", "WeatherTUI", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(logDir, "weather_app.log"), nil
}

func initLogger() *lumberjack.Logger {
	logPath, err := getLogFilePath()
	if err != nil {
		log.Fatalf("Failed to resolve log file path: %v", err)
	}

	logFile := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     30,
		Compress:   true,
		LocalTime:  true,
	}

	logger := slog.New(slog.NewJSONHandler(io.MultiWriter(logFile), &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	slog.Info("WeatherTUI service initializing")

	return logFile
}

func main() {
	logFile := initLogger()
	defer logFile.Close()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		slog.Error(fmt.Sprintf("Fatal execution crash: %v", err))
		log.Fatalf("Fatal execution crash: %v", err)
	}
}
