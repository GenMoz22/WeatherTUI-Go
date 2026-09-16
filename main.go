package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"weather-tui/internal/ui"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/natefinch/lumberjack"
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
	s.Style = lipgloss.NewStyle().Foreground(ui.Cyan)

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

		case tea.KeyMsg:
			if msg.String() == "?" && (m.activePanel != searchPanel || m.showHelp) {
				m.showHelp = !m.showHelp
				return m, nil
			}

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
										m.historySelectedRow = ui.MaxInt(0, len(m.getHistoryItems())-1)
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

	sb.WriteString(ui.HelpTitleStyle.Render("KEYBINDINGS & SYSTEM HELP") + "\n\n")

	sb.WriteString(ui.HelpSectionStyle.Render("GLOBAL CONTROLS") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("Tab"), ui.DescStyle.Render("Cycle focus between panels")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("?"), ui.DescStyle.Render("Toggle this contextual help overlay")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("Ctrl+C"), ui.DescStyle.Render("Force terminate application")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", ui.KeyStyle.Render("Esc"), ui.DescStyle.Render("Unfocus search / Exit application")))

	sb.WriteString(ui.HelpSectionStyle.Render("SEARCH PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("Enter"), ui.DescStyle.Render("Dispatch location query")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", ui.KeyStyle.Render("Text Input"), ui.DescStyle.Render("Type city or location name")))

	sb.WriteString(ui.HelpSectionStyle.Render("RECENT & FAVORITE LOCATIONS PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("j / k"), ui.DescStyle.Render("Navigate items")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("Enter"), ui.DescStyle.Render("Load selected location")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", ui.KeyStyle.Render("p"), ui.DescStyle.Render("Toggle favorite status")))

	sb.WriteString(ui.HelpSectionStyle.Render("FORECAST PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("j / k"), ui.DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("Up / Down"), ui.DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("u"), ui.DescStyle.Render("Toggle temperature unit (°C / °F)")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", ui.KeyStyle.Render("p"), ui.DescStyle.Render("Toggle active city favorite status")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", ui.KeyStyle.Render("r"), ui.DescStyle.Render("Force refresh data (bypass cache)")))

	sb.WriteString(lipgloss.NewStyle().Foreground(ui.Gray).Italic(true).Render("Press ? or Esc to return to dashboard"))

	return ui.HelpOverlayStyle.Render(sb.String())
}

func (m model) View() string {
	if m.termWidth < 80 || m.termHeight < 20 {
		return ui.SmallScreenStyle.
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

		// 1. SEARCH PANEL
		searchTitle := " COMPONENT: SEARCH ENGINE "
		searchBoxStyleToUse := ui.BoxStyle
		if m.activePanel == searchPanel {
			searchBoxStyleToUse = ui.ActiveBoxStyle
		}

		searchContent := fmt.Sprintf("%s\n\n%s", ui.PanelTitleStyle.Render(searchTitle), m.textInput.View())
		searchBox := searchBoxStyleToUse.
		Width(leftWidth - 2).
		Height(searchInnerHeight).
		MaxWidth(leftWidth - 2).
		MaxHeight(searchInnerHeight + 2).
		Render(searchContent)

		// 2. RECENT & FAVORITE LOCATIONS PANEL
		historyTitle := " RECENT & FAVORITES "
		historyBoxStyleToUse := ui.BoxStyle
		if m.activePanel == historyPanel {
			historyBoxStyleToUse = ui.ActiveBoxStyle
		}

		historyHeader := ui.PanelTitleStyle.Render(historyTitle)
		var historyContent strings.Builder
		historyContent.WriteString(historyHeader + "\n\n")

		historyItems := m.getHistoryItems()
		if len(historyItems) == 0 {
			historyContent.WriteString(lipgloss.NewStyle().Foreground(ui.Gray).Render("No recent lookups"))
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
					historyContent.WriteString(ui.SelectedRowStyle.Render(rowStr) + "\n")
				} else if isFav {
					historyContent.WriteString(ui.FavoriteStyle.Render(rowStr) + "\n")
				} else {
					historyContent.WriteString(ui.ValueStyle.Render(rowStr) + "\n")
				}
			}
		}

		historyBox := historyBoxStyleToUse.
		Width(leftWidth - 2).
		Height(historyInnerHeight).
		MaxWidth(leftWidth - 2).
		MaxHeight(historyInnerHeight + 2).
		Render(historyContent.String())

		// 3. ATMOSPHERE MONITOR PANEL
		var currentBox string

		currentHeaderTitle := " MONITOR: ATMOSPHERE "
		if m.hasData && m.cityName != "" {
			cacheBadge := ""
			if m.weather.FromCache {
				cacheBadge = lipgloss.NewStyle().Foreground(ui.Yellow).Bold(true).Render(" [CACHE HIT]")
			}
			favBadge := ""
			if strings.EqualFold(m.lastQuery, m.favoriteCity) {
				favBadge = ui.FavoriteStyle.Render(" ★")
			}
			currentHeaderTitle = fmt.Sprintf(" MONITOR: %s%s%s ", m.cityName, favBadge, cacheBadge)
		}
		currentHeader := ui.PanelTitleStyle.Render(currentHeaderTitle)

		currentStyle := ui.BoxStyle
		if m.err != nil {
			currentStyle = ui.ErrorBoxStyle
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
				aqiDesc = lipgloss.NewStyle().Foreground(ui.Green).Render(fmt.Sprintf("%d (EXCELLENT)", aqiVal))
			} else if aqiVal <= 40 {
				aqiDesc = lipgloss.NewStyle().Foreground(ui.Yellow).Render(fmt.Sprintf("%d (POOR)", aqiVal))
			} else {
				aqiDesc = lipgloss.NewStyle().Foreground(ui.Red).Render(fmt.Sprintf("%d (CRITICAL)", aqiVal))
			}

			rawCelsius := m.weather.Current.Temperature
			tempVal := rawCelsius
			unitStr := "°C"
			if m.useFahrenheit {
				tempVal = ui.CelsiusToFahrenheit(rawCelsius)
				unitStr = "°F"
			}

			tempStyle := ui.GetTemperatureStyle(rawCelsius)
			renderedTemp := tempStyle.Render(fmt.Sprintf("%.1f %s", tempVal, unitStr))

			humidityStyle := ui.GetHumidityStyle(m.weather.Current.Humidity)
			renderedHumidity := humidityStyle.Render(fmt.Sprintf("%d%%", m.weather.Current.Humidity))

			windStyle := ui.GetWindSpeedStyle(m.weather.Current.WindSpeed)
			windDirCompass := ui.DegreesToCompass(m.weather.Current.WindDirection)
			windStr := fmt.Sprintf("%.1f km/h %s (%.0f°)", m.weather.Current.WindSpeed, windDirCompass, m.weather.Current.WindDirection)
			renderedWind := windStyle.Render(windStr)

			uvDesc := ui.GetUVIndexDesc(m.weather.Current.UvIndex)

			metricsContent := fmt.Sprintf(
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s",
				 ui.LabelStyle.Render("[TEMP]"), "Temperature:", renderedTemp,
						      ui.LabelStyle.Render("[HUMI]"), "Humidity:", renderedHumidity,
						      ui.LabelStyle.Render("[WIND]"), "Wind:", renderedWind,
						      ui.LabelStyle.Render("[UVIN]"), "UV Index:", uvDesc,
						      ui.LabelStyle.Render("[SUNR]"), "Sun Rise:", lipgloss.NewStyle().Foreground(ui.Blue).Render(m.weather.Current.Sunrise),
						      ui.LabelStyle.Render("[SUNS]"), "Sun Set:", lipgloss.NewStyle().Foreground(ui.Blue).Render(m.weather.Current.Sunset),
						      ui.LabelStyle.Render("[AQI ]"), "Air Quality:", aqiDesc,
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
				placeholderText = lipgloss.NewStyle().Foreground(ui.Red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.err))
			}
			currentBox = currentStyle.
			Width(leftWidth - 2).
			Height(currentInnerHeight).
			MaxWidth(leftWidth - 2).
			MaxHeight(currentInnerHeight + 2).
			Render(currentHeader + "\n\n" + placeholderText)
		}

		leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, historyBox, currentBox)

		// 4. 14-DAY FORECAST PANEL
		var forecastBox string
		forecastHeader := ui.PanelTitleStyle.Render(" METRIC: 14-DAY CORE FORECAST ") + "\n\n"

			forecastBoxStyleToUse := ui.BoxStyle
				if m.activePanel == forecastPanel {
					forecastBoxStyleToUse = ui.ActiveBoxStyle
				}

				const fixedTableColumnsWidth = 36
				dynamicBarLength := rightWidth - fixedTableColumnsWidth - 9
				if dynamicBarLength < 10 {
					dynamicBarLength = 10
				}

				graphHeaderPadding := strings.Repeat(" ", ui.MaxInt(0, dynamicBarLength-15))
				unitHeader := "MAX (°C)│ MIN (°C)"
				if m.useFahrenheit {
					unitHeader = "MAX (°F)│ MIN (°F)"
				}
				tableHeader := lipgloss.NewStyle().Foreground(ui.LightGray).Render(fmt.Sprintf(" DATE       │ %s │ PRECIPITATION GRAPH%s", unitHeader, graphHeaderPadding)) + "\n"
				tableDivider := lipgloss.NewStyle().Foreground(ui.Gray).Render(strings.Repeat("─", ui.MaxInt(10, rightWidth-4))) + "\n"

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
							barStyle = lipgloss.NewStyle().Foreground(ui.Red)
						} else if day.PrecipProbability > 30 {
							barStyle = lipgloss.NewStyle().Foreground(ui.Yellow)
						} else {
							barStyle = lipgloss.NewStyle().Foreground(ui.Gray)
						}

						renderedBar := barStyle.Render(fmt.Sprintf("%s %3d%%", barStr.String(), day.PrecipProbability))

						maxTemp := day.MaxTemp
						minTemp := day.MinTemp
						if m.useFahrenheit {
							maxTemp = ui.CelsiusToFahrenheit(maxTemp)
							minTemp = ui.CelsiusToFahrenheit(minTemp)
						}

						maxStyle := ui.GetTemperatureStyle(day.MaxTemp)
						minStyle := ui.GetTemperatureStyle(day.MinTemp)

						renderedMax := maxStyle.Render(fmt.Sprintf("%-7.1f", maxTemp))
						renderedMin := minStyle.Render(fmt.Sprintf("%-7.1f", minTemp))

						row := fmt.Sprintf(" %-10s │  %s │  %s │ %s", day.Date, renderedMax, renderedMin, renderedBar)

						if m.activePanel == forecastPanel && i == m.selectedRow {
							row = ui.SelectedRowStyle.Render(row)
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

				// 5. FOOTER STATUS BAR
				navKeys := ui.KeyStyle.Render("Tab") + ui.DescStyle.Render("Switch View")

				if m.activePanel == historyPanel {
					navKeys += ui.KeyStyle.Render("j/k") + ui.DescStyle.Render("Navigate") +
					ui.KeyStyle.Render("p") + ui.DescStyle.Render("Favorite") +
					ui.KeyStyle.Render("Enter") + ui.DescStyle.Render("Load")
				} else if m.activePanel == forecastPanel {
					navKeys += ui.KeyStyle.Render("u") + ui.DescStyle.Render("Toggle °C/°F") +
					ui.KeyStyle.Render("p") + ui.DescStyle.Render("Favorite") +
					ui.KeyStyle.Render("r") + ui.DescStyle.Render("Refresh") +
					ui.KeyStyle.Render("j/k") + ui.DescStyle.Render("Rows")
				}

				var statusElements []string
				if m.activePanel == searchPanel {
					statusElements = append(statusElements, ui.KeyStyle.Render("Enter"), ui.DescStyle.Render("Search"))
				}
				statusElements = append(statusElements, navKeys)

				if m.activePanel != searchPanel {
					statusElements = append(statusElements, ui.KeyStyle.Render("?"), ui.DescStyle.Render("Help"))
				}

				if m.activePanel == searchPanel {
					statusElements = append(statusElements, ui.KeyStyle.Render("Esc"), ui.DescStyle.Render("Unfocus"))
					statusElements = append(statusElements, ui.KeyStyle.Render("Ctrl+C"), ui.DescStyle.Render("Exit"))
				} else {
					statusElements = append(statusElements, ui.KeyStyle.Render("Esc / Ctrl+C"), ui.DescStyle.Render("Exit"))
				}

				statusElements = append(statusElements,
							lipgloss.NewStyle().Foreground(ui.Gray).Padding(0, 1).Render("│"),
							lipgloss.NewStyle().Foreground(ui.Cyan).Italic(true).Render(fmt.Sprintf("WeatherTUI - [res: %dx%d]", m.termWidth, m.termHeight)),
				)

				statusBar := lipgloss.JoinHorizontal(lipgloss.Left, statusElements...)

				fullLayout := lipgloss.JoinVertical(lipgloss.Left, mainDashboard, statusBar)

				return lipgloss.NewStyle().
				MaxWidth(m.termWidth).
				MaxHeight(m.termHeight).
				Render("\n" + fullLayout + "\n")
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
