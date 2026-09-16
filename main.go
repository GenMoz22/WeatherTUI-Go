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
	favoriteStyle    = lipgloss.NewStyle().Foreground(yellow).Bold(true)

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

	// Help overlay modal styles
	helpOverlayStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(cyan).
	Background(darkBg).
	Padding(1, 2)
	helpTitleStyle   = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	helpSectionStyle = lipgloss.NewStyle().Foreground(yellow).Bold(true)

	// Fallback style for low-resolution screens
	smallScreenStyle = lipgloss.NewStyle().
	Foreground(yellow).
	Bold(true).
	Align(lipgloss.Center, lipgloss.Center)
)

type activePanel int

const (
	searchPanel activePanel = iota
	historyPanel
	forecastPanel
)

type errMsg error

type model struct {
	textInput          textinput.Model
	spinner            spinner.Model
	client             *WeatherClient
	configMgr          *ConfigManager
	weather            WeatherResponse
	cityName           string
	lastQuery          string
	favoriteCity       string
	recentLocations    []string
	historySelectedRow int
	err                error
	loading            bool
	hasData            bool
	useFahrenheit      bool
	activePanel        activePanel
	selectedRow        int
	termWidth          int
	termHeight         int
	showHelp           bool
	initialCity        string
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

	cfgMgr, err := NewConfigManager()
	if err != nil {
		slog.Error("Failed initializing ConfigManager", "error", err)
	}

	useFahrenheit := false
	favoriteCity := ""
	recentLocations := make([]string, 0)

	if cfgMgr != nil {
		cfg, err := cfgMgr.Load()
		if err == nil {
			useFahrenheit = cfg.UseFahrenheit
			favoriteCity = cfg.FavoriteCity
			recentLocations = cfg.RecentLocations
		}
	}

	// Resolve startup target: priority given to favorite city, falling back to recent location
	initialCity := strings.TrimSpace(favoriteCity)
	if initialCity == "" && len(recentLocations) > 0 {
		initialCity = strings.TrimSpace(recentLocations[0])
	}

	return model{
		textInput:          ti,
		spinner:            s,
		client:             NewWeatherClient(),
		configMgr:          cfgMgr,
		activePanel:        searchPanel,
		useFahrenheit:      useFahrenheit,
		favoriteCity:       favoriteCity,
		recentLocations:    recentLocations,
		selectedRow:        0,
		historySelectedRow: 0,
		termWidth:          100,
		termHeight:         24,
		showHelp:           false,
		initialCity:        initialCity,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink, m.spinner.Tick}

	// Trigger initial weather fetch command automatically if a default city is configured
	if m.initialCity != "" {
		m.loading = true
		m.lastQuery = m.initialCity
		cmds = append(cmds, m.fetchWeatherCmd(m.initialCity, false))
	}

	return tea.Batch(cmds...)
}

type weatherMsg struct {
	data  string
	query string
	w     WeatherResponse
}

func celsiusToFahrenheit(c float64) float64 {
	return (c * 9 / 5) + 32
}

func degreesToCompass(degrees float64) string {
	directions := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	index := int((degrees + 11.25) / 22.5)
	return directions[index%16]
}

func getUVIndexDesc(uv float64) string {
	if uv <= 2 {
		return lipgloss.NewStyle().Foreground(green).Render(fmt.Sprintf("%.1f (LOW)", uv))
	} else if uv <= 5 {
		return lipgloss.NewStyle().Foreground(yellow).Render(fmt.Sprintf("%.1f (MODERATE)", uv))
	} else if uv <= 7 {
		return lipgloss.NewStyle().Foreground(yellow).Bold(true).Render(fmt.Sprintf("%.1f (HIGH)", uv))
	} else if uv <= 10 {
		return lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("%.1f (VERY HIGH)", uv))
	}
	return lipgloss.NewStyle().Foreground(red).Bold(true).Render(fmt.Sprintf("%.1f (EXTREME)", uv))
}

func (m model) fetchWeatherCmd(city string, forceRefresh bool) tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			w, name, err := m.client.GetWeather(city, forceRefresh)
			if err != nil {
				return errMsg(err)
			}
			return weatherMsg{data: name, query: city, w: w}
		},
	)
}

func (m *model) saveConfig() {
	if m.configMgr == nil {
		return
	}
	cfg := Config{
		UseFahrenheit:   m.useFahrenheit,
		FavoriteCity:    m.favoriteCity,
		RecentLocations: m.recentLocations,
	}
	if err := m.configMgr.Save(cfg); err != nil {
		slog.Error("Failed persisting configuration", "error", err)
	}
}

func (m *model) addRecentLocation(location string) {
	m.recentLocations, m.favoriteCity = UpdateRecentAndFavorite(m.recentLocations, m.favoriteCity, location)
	m.saveConfig()
}

func (m *model) toggleFavorite(targetCity string) {
	cleanLoc := strings.TrimSpace(targetCity)
	if cleanLoc == "" {
		return
	}

	if strings.EqualFold(m.favoriteCity, cleanLoc) {
		m.favoriteCity = ""
	} else {
		m.favoriteCity = cleanLoc
		// Purge the favorited location from recent locations list to keep entries strictly unique
		filtered := make([]string, 0, len(m.recentLocations))
		for _, loc := range m.recentLocations {
			if !strings.EqualFold(loc, cleanLoc) {
				filtered = append(filtered, loc)
			}
		}
		m.recentLocations = filtered
	}
	m.saveConfig()
}

func (m model) getHistoryItems() []string {
	items := make([]string, 0, 4)
	if m.favoriteCity != "" {
		items = append(items, m.favoriteCity)
	}
	items = append(items, m.recentLocations...)
	return items
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.termWidth = msg.Width
			m.termHeight = msg.Height
			return m, nil

		case tea.KeyMsg:
			// Toggle help overlay on '?' only when not typing inside search input panel or when overlay is open.
			if msg.String() == "?" && (m.activePanel != searchPanel || m.showHelp) {
				m.showHelp = !m.showHelp
				return m, nil
			}

			// When help overlay is active, Esc or 'q' closes the overlay modal.
			if m.showHelp {
				switch msg.Type {
					case tea.KeyEsc, tea.KeyCtrlC:
						m.showHelp = false
						return m, nil
					case tea.KeyRunes:
						if msg.String() == "q" || msg.String() == "Q" {
							m.showHelp = false
							return m, nil
						}
				}
				return m, nil
			}

			switch msg.Type {
				case tea.KeyCtrlC:
					return m, tea.Quit

				case tea.KeyEsc:
					if m.activePanel == searchPanel {
						m.activePanel = forecastPanel
						m.textInput.Blur()
						return m, nil
					}
					return m, tea.Quit

				case tea.KeyTab:
					if m.activePanel == searchPanel {
						m.activePanel = historyPanel
						m.textInput.Blur()
					} else if m.activePanel == historyPanel {
						m.activePanel = forecastPanel
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

					if m.activePanel == historyPanel {
						historyItems := m.getHistoryItems()
						if len(historyItems) > 0 && m.historySelectedRow < len(historyItems) {
							selectedCity := historyItems[m.historySelectedRow]
							m.loading = true
							m.err = nil
							m.lastQuery = selectedCity
							return m, m.fetchWeatherCmd(selectedCity, false)
						}
					}

				case tea.KeyUp, tea.KeyDown, tea.KeyRunes:
					key := msg.String()

					if m.activePanel == historyPanel {
						historyItems := m.getHistoryItems()
						if len(historyItems) > 0 {
							switch key {
								case "k", "up":
									if m.historySelectedRow > 0 {
										m.historySelectedRow--
									}
									return m, nil
								case "j", "down":
									if m.historySelectedRow < len(historyItems)-1 {
										m.historySelectedRow++
									}
									return m, nil
								case "p", "P":
									selectedCity := historyItems[m.historySelectedRow]
									m.toggleFavorite(selectedCity)
									if m.historySelectedRow >= len(m.getHistoryItems()) {
										m.historySelectedRow = maxInt(0, len(m.getHistoryItems())-1)
									}
									return m, nil
							}
						}
					}

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
								case "p", "P":
									if m.lastQuery != "" {
										m.toggleFavorite(m.lastQuery)
									}
									return m, nil
							}
						}

						if key == "u" || key == "U" {
							m.useFahrenheit = !m.useFahrenheit
							m.saveConfig()
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
									m.addRecentLocation(msg.query)
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

	if m.activePanel == searchPanel && !m.showHelp {
		var inputCmd tea.Cmd
		m.textInput, inputCmd = m.textInput.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) renderHelpOverlay() string {
	var sb strings.Builder

	sb.WriteString(helpTitleStyle.Render("KEYBINDINGS & SYSTEM HELP") + "\n\n")

	sb.WriteString(helpSectionStyle.Render("GLOBAL CONTROLS") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("Tab"), descStyle.Render("Cycle focus between panels")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("?"), descStyle.Render("Toggle this contextual help overlay")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("Ctrl+C"), descStyle.Render("Force terminate application")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", keyStyle.Render("Esc"), descStyle.Render("Unfocus search / Exit application")))

	sb.WriteString(helpSectionStyle.Render("SEARCH PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("Enter"), descStyle.Render("Dispatch location query")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", keyStyle.Render("Text Input"), descStyle.Render("Type city or location name")))

	sb.WriteString(helpSectionStyle.Render("RECENT & FAVORITE LOCATIONS PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("j / k"), descStyle.Render("Navigate items")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("Enter"), descStyle.Render("Load selected location")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", keyStyle.Render("p"), descStyle.Render("Toggle favorite status")))

	sb.WriteString(helpSectionStyle.Render("FORECAST PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("j / k"), descStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("Up / Down"), descStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("u"), descStyle.Render("Toggle temperature unit (°C / °F)")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", keyStyle.Render("p"), descStyle.Render("Toggle active city favorite status")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", keyStyle.Render("r"), descStyle.Render("Force refresh data (bypass cache)")))

	sb.WriteString(lipgloss.NewStyle().Foreground(gray).Italic(true).Render("Press ? or Esc to return to dashboard"))

	return helpOverlayStyle.Render(sb.String())
}

func (m model) View() string {
	// Defensive screen resolution guard against vertical/horizontal clipping
	if m.termWidth < 80 || m.termHeight < 20 {
		return smallScreenStyle.
		Width(m.termWidth).
		Height(m.termHeight).
		Render("Terminal screen too small. Please resize.")
	}

	availableHeight := m.termHeight - 4
	if availableHeight < 16 {
		availableHeight = 16
	}

	leftWidth := int(float64(m.termWidth) * 0.35)
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.termWidth - leftWidth - 2
	if rightWidth < 40 {
		rightWidth = 40
	}

	const searchBoxHeight = 5
	historyBoxHeight := 8
	currentBoxHeight := availableHeight - searchBoxHeight - historyBoxHeight
	if currentBoxHeight < 9 {
		currentBoxHeight = 9
		historyBoxHeight = availableHeight - searchBoxHeight - currentBoxHeight
	}

	searchInnerHeight := searchBoxHeight - 2
	historyInnerHeight := historyBoxHeight - 2
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
		searchBox := searchBoxStyleToUse.
		Width(leftWidth - 2).
		Height(searchInnerHeight).
		MaxWidth(leftWidth - 2).
		MaxHeight(searchInnerHeight + 2).
		Render(searchContent)

		// ------------------------------------------------------------------------
		// 2. RECENT & FAVORITE LOCATIONS PANEL
		// ------------------------------------------------------------------------
		historyTitle := " RECENT & FAVORITES "
		historyBoxStyleToUse := boxStyle
		if m.activePanel == historyPanel {
			historyBoxStyleToUse = activeBoxStyle
		}

		historyHeader := panelTitleStyle.Render(historyTitle)
		var historyContent strings.Builder
		historyContent.WriteString(historyHeader + "\n\n")

		historyItems := m.getHistoryItems()
		if len(historyItems) == 0 {
			historyContent.WriteString(lipgloss.NewStyle().Foreground(gray).Render("No recent lookups"))
		} else {
			maxVisibleRows := historyInnerHeight - 2
			if maxVisibleRows < 1 {
				maxVisibleRows = 1
			}

			startIdx := 0
			if m.historySelectedRow >= maxVisibleRows {
				startIdx = m.historySelectedRow - maxVisibleRows + 1
			}
			endIdx := startIdx + maxVisibleRows
			if endIdx > len(historyItems) {
				endIdx = len(historyItems)
			}

			for i := startIdx; i < endIdx; i++ {
				loc := historyItems[i]
				isFav := strings.EqualFold(loc, m.favoriteCity)

				displayStr := loc
				if isFav {
					displayStr = "★ " + loc
				} else {
					displayStr = "  " + loc
				}

				rowStr := fmt.Sprintf(" %-30s", displayStr)
				if len(rowStr) > leftWidth-6 {
					rowStr = rowStr[:leftWidth-6]
				}

				if m.activePanel == historyPanel && i == m.historySelectedRow {
					historyContent.WriteString(selectedRowStyle.Render(rowStr) + "\n")
				} else if isFav {
					historyContent.WriteString(favoriteStyle.Render(rowStr) + "\n")
				} else {
					historyContent.WriteString(valueStyle.Render(rowStr) + "\n")
				}
			}
		}

		historyBox := historyBoxStyleToUse.
		Width(leftWidth - 2).
		Height(historyInnerHeight).
		MaxWidth(leftWidth - 2).
		MaxHeight(historyInnerHeight + 2).
		Render(historyContent.String())

		// ------------------------------------------------------------------------
		// 3. ATMOSPHERE MONITOR PANEL
		// ------------------------------------------------------------------------
		var currentBox string

		currentHeaderTitle := " MONITOR: ATMOSPHERE "
		if m.hasData && m.cityName != "" {
			cacheBadge := ""
			if m.weather.FromCache {
				cacheBadge = lipgloss.NewStyle().Foreground(yellow).Bold(true).Render(" [CACHE HIT]")
			}
			favBadge := ""
			if strings.EqualFold(m.lastQuery, m.favoriteCity) {
				favBadge = favoriteStyle.Render(" ★")
			}
			currentHeaderTitle = fmt.Sprintf(" MONITOR: %s%s%s ", m.cityName, favBadge, cacheBadge)
		}
		currentHeader := panelTitleStyle.Render(currentHeaderTitle)

		currentStyle := boxStyle
		if m.err != nil {
			currentStyle = errorBoxStyle
		}

		if m.loading {
			loadingText := fmt.Sprintf("%s Fetching telemetry pipeline...", m.spinner.View())
			placeholderText := fmt.Sprintf("STATUS: PARSING METRICS...\n\n%s\nSynchronizing Open-Meteo DB\nStream pipelines active...", loadingText)
			currentBox = currentStyle.
			Width(leftWidth - 2).
			Height(currentInnerHeight).
			MaxWidth(leftWidth - 2).
			MaxHeight(currentInnerHeight + 2).
			Render(currentHeader + "\n\n" + placeholderText)
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

			uvDesc := getUVIndexDesc(m.weather.Current.UvIndex)
			windDirCompass := degreesToCompass(m.weather.Current.WindDirection)
			windDirStr := fmt.Sprintf("%s (%.0f°)", windDirCompass, m.weather.Current.WindDirection)

			metricsContent := fmt.Sprintf(
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s",
				 labelStyle.Render("[TEMP]"), "Temperature:", highlightStyle.Render(fmt.Sprintf("%.1f %s", tempVal, unitStr)),
						      labelStyle.Render("[HUMI]"), "Humidity:", valueStyle.Render(fmt.Sprintf("%d%%", m.weather.Current.Humidity)),
						      labelStyle.Render("[WIND]"), "Wind Speed:", valueStyle.Render(fmt.Sprintf("%.1f km/h", m.weather.Current.WindSpeed)),
						      labelStyle.Render("[WDIR]"), "Wind Dir:", valueStyle.Render(windDirStr),
						      labelStyle.Render("[UVIN]"), "UV Index:", uvDesc,
						      labelStyle.Render("[SUNR]"), "Sun Rise:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunrise),
						      labelStyle.Render("[SUNS]"), "Sun Set:", lipgloss.NewStyle().Foreground(blue).Render(m.weather.Current.Sunset),
						      labelStyle.Render("[AQI ]"), "Air Quality:", aqiDesc,
			)

			fullCurrentView := currentHeader + "\n\n" + lipgloss.PlaceVertical(currentInnerHeight-3, lipgloss.Top, metricsContent)
			currentBox = currentStyle.
			Width(leftWidth - 2).
			Height(currentInnerHeight).
			MaxWidth(leftWidth - 2).
			MaxHeight(currentInnerHeight + 2).
			Render(fullCurrentView)
		} else {
			placeholderText := "STATUS: SYSTEM IDLE\n\nAwaiting dispatcher query...\nInsert location name above."
			if m.err != nil {
				placeholderText = lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.err))
			}
			currentBox = currentStyle.
			Width(leftWidth - 2).
			Height(currentInnerHeight).
			MaxWidth(leftWidth - 2).
			MaxHeight(currentInnerHeight + 2).
			Render(currentHeader + "\n\n" + placeholderText)
		}

		leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, historyBox, currentBox)

		// ------------------------------------------------------------------------
		// 4. 14-DAY FORECAST PANEL
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

					forecastBox = forecastBoxStyleToUse.
						Width(rightWidth).
						Height(forecastInnerHeight).
						MaxWidth(rightWidth).
						MaxHeight(forecastInnerHeight + 2).
						Render(forecastHeader + tableHeader + tableDivider + strings.Join(contentLines, "\n"))
				} else {
					statusMsg := "[WAIT] Pipeline awaiting telemetry input..."
					if m.loading {
						statusMsg = fmt.Sprintf("%s Streaming telemetry from Open-Meteo clusters...", m.spinner.View())
					}
					emptyLines := fmt.Sprintf("\n\n\n\n\n\n\n          %s", statusMsg)
					forecastBox = forecastBoxStyleToUse.
						Width(rightWidth).
						Height(forecastInnerHeight).
						MaxWidth(rightWidth).
						MaxHeight(forecastInnerHeight + 2).
						Render(forecastHeader + tableHeader + tableDivider + emptyLines)
				}

				mainDashboard := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, forecastBox)

				// Overlay modal layout when help toggle is active
				if m.showHelp {
					helpModal := m.renderHelpOverlay()
					mainDashboard = lipgloss.Place(
						m.termWidth,
				    m.termHeight-2,
				    lipgloss.Center,
				    lipgloss.Center,
				    helpModal,
				    lipgloss.WithWhitespaceChars(" "),
					)
				}

				// ------------------------------------------------------------------------
				// 5. FOOTER STATUS BAR
				// ------------------------------------------------------------------------
				navKeys := keyStyle.Render("Tab") + descStyle.Render("Switch View")

				if m.activePanel == historyPanel {
					navKeys += keyStyle.Render("j/k") + descStyle.Render("Navigate") +
					keyStyle.Render("p") + descStyle.Render("Favorite") +
					keyStyle.Render("Enter") + descStyle.Render("Load")
				} else if m.activePanel == forecastPanel {
					navKeys += keyStyle.Render("u") + descStyle.Render("Toggle °C/°F") +
					keyStyle.Render("p") + descStyle.Render("Favorite") +
					keyStyle.Render("r") + descStyle.Render("Refresh") +
					keyStyle.Render("j/k") + descStyle.Render("Rows")
				}

				var statusElements []string
				if m.activePanel == searchPanel {
					statusElements = append(statusElements, keyStyle.Render("Enter"), descStyle.Render("Search"))
				}
				statusElements = append(statusElements, navKeys)

				// Hide Help key legend when active panel is searchPanel to avoid displaying '?' while typing.
				if m.activePanel != searchPanel {
					statusElements = append(statusElements, keyStyle.Render("?"), descStyle.Render("Help"))
				}

				if m.activePanel == searchPanel {
					statusElements = append(statusElements, keyStyle.Render("Esc"), descStyle.Render("Unfocus"))
					statusElements = append(statusElements, keyStyle.Render("Ctrl+C"), descStyle.Render("Exit"))
				} else {
					statusElements = append(statusElements, keyStyle.Render("Esc / Ctrl+C"), descStyle.Render("Exit"))
				}

				statusElements = append(statusElements,
							lipgloss.NewStyle().Foreground(gray).Padding(0, 1).Render("│"),
							lipgloss.NewStyle().Foreground(cyan).Italic(true).Render(fmt.Sprintf("WeatherTUI - [res: %dx%d]", m.termWidth, m.termHeight)),
				)

				statusBar := lipgloss.JoinHorizontal(lipgloss.Left, statusElements...)

				fullLayout := lipgloss.JoinVertical(lipgloss.Left, mainDashboard, statusBar)

				return lipgloss.NewStyle().
				MaxWidth(m.termWidth).
				MaxHeight(m.termHeight).
				Render("\n" + fullLayout + "\n")
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
