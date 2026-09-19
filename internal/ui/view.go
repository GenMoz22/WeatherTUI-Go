package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderHelpOverlay() string {
	var sb strings.Builder

	sb.WriteString(HelpTitleStyle.Render("KEYBINDINGS & SYSTEM HELP") + "\n\n")

	sb.WriteString(HelpSectionStyle.Render("GLOBAL CONTROLS") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Tab"), DescStyle.Render("Cycle focus between panels")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("1 / 2 / 3"), DescStyle.Render("Direct jump to Search / Recent / Forecast panel")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("?"), DescStyle.Render("Toggle this contextual help overlay")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Ctrl+C"), DescStyle.Render("Force terminate application")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("Esc"), DescStyle.Render("Unfocus search / Exit application")))

	sb.WriteString(HelpSectionStyle.Render("SEARCH PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Enter"), DescStyle.Render("Dispatch location query")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("Text Input"), DescStyle.Render("Type city or location name")))

	sb.WriteString(HelpSectionStyle.Render("RECENT & FAVORITES PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("j / k"), DescStyle.Render("Navigate items")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Enter"), DescStyle.Render("Load selected location")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("p"), DescStyle.Render("Toggle favorite status")))

	sb.WriteString(HelpSectionStyle.Render("FORECAST PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("j / k"), DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Up / Down"), DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("v"), DescStyle.Render("Toggle 24h hourly trend sparkline view")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("u"), DescStyle.Render("Toggle temperature unit (°C / °F)")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("p"), DescStyle.Render("Toggle active city favorite status")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("r"), DescStyle.Render("Force refresh data (bypass cache)")))

	sb.WriteString(lipgloss.NewStyle().Foreground(Gray).Italic(true).Render("Press ? or Esc to return to dashboard"))

	return HelpOverlayStyle.Render(sb.String())
}

// View renders the application user interface
func (m Model) View() string {
	if m.TermWidth < 80 || m.TermHeight < 20 {
		return SmallScreenStyle.
		Width(m.TermWidth).
		Height(m.TermHeight).
		Render("Terminal screen too small. Please resize.")
	}

	availableHeight := m.TermHeight - 2
	if availableHeight < 16 {
		availableHeight = 16
	}

	leftWidth := int(float64(m.TermWidth) * 0.38)
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.TermWidth - leftWidth - 1
	if rightWidth < 42 {
		rightWidth = 42
	}

	const searchBoxHeight = 3
	historyBoxHeight := 7
	currentBoxHeight := availableHeight - searchBoxHeight - historyBoxHeight
	if currentBoxHeight < 10 {
		currentBoxHeight = 10
		historyBoxHeight = availableHeight - searchBoxHeight - currentBoxHeight
	}

	// 1. SEARCH PANEL
	searchContent := m.TextInput.View()
	searchBox := RenderPanelBox("1. Search Engine", searchContent, leftWidth, searchBoxHeight, m.ActivePanel == SearchPanel, false)

	// 2. RECENT & FAVORITES PANEL
	var historyContent strings.Builder
	historyItems := m.GetHistoryItems()

	if len(historyItems) == 0 {
		historyContent.WriteString(lipgloss.NewStyle().Foreground(Gray).Render("No recent lookups"))
	} else {
		maxVisibleRows := historyBoxHeight - 2
		if maxVisibleRows < 1 {
			maxVisibleRows = 1
		}

		startIdx := 0
		if m.HistorySelectedRow >= maxVisibleRows {
			startIdx = m.HistorySelectedRow - maxVisibleRows + 1
		}
		endIdx := startIdx + maxVisibleRows
		if endIdx > len(historyItems) {
			endIdx = len(historyItems)
		}

		for i := startIdx; i < endIdx; i++ {
			loc := historyItems[i]
			isFav := strings.EqualFold(loc, m.FavoriteCity)

			prefix := "      "
			if isFav {
				prefix = "[FAV] "
			}

			displayStr := prefix + loc
			if len(displayStr) > leftWidth-6 {
				displayStr = displayStr[:leftWidth-6]
			}

			if m.ActivePanel == HistoryPanel && i == m.HistorySelectedRow {
				rowStr := fmt.Sprintf("▌ %-*s", leftWidth-8, displayStr)
				historyContent.WriteString(SelectedRowStyle.Render(rowStr) + "\n")
			} else if isFav {
				rowStr := fmt.Sprintf("  %-*s", leftWidth-8, displayStr)
				historyContent.WriteString(FavoriteStyle.Render(rowStr) + "\n")
			} else {
				rowStr := fmt.Sprintf("  %-*s", leftWidth-8, displayStr)
				historyContent.WriteString(ValueStyle.Render(rowStr) + "\n")
			}
		}
	}

	historyBox := RenderPanelBox("2. Recent & Favorites", historyContent.String(), leftWidth, historyBoxHeight, m.ActivePanel == HistoryPanel, false)

	// 3. ATMOSPHERE MONITOR PANEL
	currentTitle := "Monitor: Atmosphere"
	displayName := m.CityName
	if displayName == "" && m.HasData {
		displayName = m.Weather.CityName
	}

	if m.HasData && displayName != "" {
		cacheBadge := ""
		if m.Weather.FromCache {
			cacheBadge = " [CACHE]"
		}
		favBadge := ""
		if strings.EqualFold(displayName, m.FavoriteCity) || strings.EqualFold(m.LastQuery, m.FavoriteCity) {
			favBadge = " [FAV]"
		}

		fullTitle := fmt.Sprintf("Monitor: %s%s%s", displayName, favBadge, cacheBadge)
		maxTitleLen := leftWidth - 4
		if maxTitleLen > 0 && lipgloss.Width(fullTitle) > maxTitleLen {
			currentTitle = lipgloss.NewStyle().MaxWidth(maxTitleLen).Render(fullTitle)
		} else {
			currentTitle = fullTitle
		}
	}

	var currentInnerContent string
	if m.Loading {
		loadingText := fmt.Sprintf("%s Fetching telemetry pipeline...", m.Spinner.View())
		currentInnerContent = fmt.Sprintf("STATUS: PARSING METRICS...\n\n%s\nSynchronizing Open-Meteo DB", loadingText)
	} else if m.HasData {
		aqiVal := m.Weather.Current.AirQualityIndex
		var aqiDesc string
		if aqiVal <= 20 {
			aqiDesc = lipgloss.NewStyle().Foreground(Green).Bold(true).Render(fmt.Sprintf("%d (EXCELLENT)", aqiVal))
		} else if aqiVal <= 40 {
			aqiDesc = lipgloss.NewStyle().Foreground(Yellow).Bold(true).Render(fmt.Sprintf("%d (POOR)", aqiVal))
		} else {
			aqiDesc = lipgloss.NewStyle().Foreground(Red).Bold(true).Render(fmt.Sprintf("%d (CRITICAL)", aqiVal))
		}

		rawCelsius := m.Weather.Current.Temperature
		tempVal := rawCelsius
		unitStr := "°C"
		if m.UseFahrenheit {
			tempVal = CelsiusToFahrenheit(rawCelsius)
			unitStr = "°F"
		}

		tempStyle := GetTemperatureStyle(rawCelsius)
		renderedTemp := tempStyle.Render(fmt.Sprintf("%.1f %s", tempVal, unitStr))

		humidityStyle := GetHumidityStyle(m.Weather.Current.Humidity)
		renderedHumidity := humidityStyle.Render(fmt.Sprintf("%d%%", m.Weather.Current.Humidity))

		windStyle := GetWindSpeedStyle(m.Weather.Current.WindSpeed)
		windDirCompass := DegreesToCompass(m.Weather.Current.WindDirection)
		windStr := fmt.Sprintf("%.1f km/h %s (%.0f°)", m.Weather.Current.WindSpeed, windDirCompass, m.Weather.Current.WindDirection)
		renderedWind := windStyle.Render(windStr)

		uvDesc := GetUVIndexDesc(m.Weather.Current.UvIndex)
		moonStr := fmt.Sprintf("%s %d%%", m.Weather.Current.MoonPhase.PhaseIcon, m.Weather.Current.MoonPhase.IlluminationPercent)
		renderedMoon := lipgloss.NewStyle().Foreground(Yellow).Bold(true).Render(moonStr)

		const labelWidth = 13
		innerMaxW := leftWidth - 4
		if innerMaxW < 20 {
			innerMaxW = 20
		}

		formatRow := func(label, value string) string {
			l := LabelStyle.Render(label)
			row := fmt.Sprintf("%-*s %s", labelWidth, l, value)
			if lipgloss.Width(row) > innerMaxW {
				return lipgloss.NewStyle().MaxWidth(innerMaxW).Render(row)
			}
			return row
		}

		currentInnerContent = fmt.Sprintf(
			"%s\n%s\n%s\n%s\n%s\n%s\n%s\n%s",
			formatRow("Temperature:", renderedTemp),
				formatRow("Humidity:", renderedHumidity),
					formatRow("Wind:", renderedWind),
						formatRow("UV Index:", uvDesc),
							formatRow("Moon Phase:", renderedMoon),
								formatRow("Sunrise:", lipgloss.NewStyle().Foreground(Blue).Bold(true).Render(m.Weather.Current.Sunrise)),
									formatRow("Sunset:", lipgloss.NewStyle().Foreground(Blue).Bold(true).Render(m.Weather.Current.Sunset)),
										formatRow("Air Quality:", aqiDesc),
		)
	} else {
		currentInnerContent = "STATUS: SYSTEM IDLE\n\nAwaiting dispatcher query..."
		if m.Err != nil {
			currentInnerContent = lipgloss.NewStyle().Foreground(Red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.Err))
		}
	}

	// Active focus styling logic for Monitor panel
	isMonitorActive := m.ActivePanel != SearchPanel && m.ActivePanel != HistoryPanel && m.ActivePanel != ForecastPanel
	currentBox := RenderPanelBox(currentTitle, currentInnerContent, leftWidth, currentBoxHeight, isMonitorActive, m.Err != nil)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left, searchBox, historyBox, currentBox)

	// 4. FORECAST PANEL / HOURLY TREND PANEL
	var forecastTitle string
	var forecastInnerContent string

	if m.ShowHourly && m.HasData {
		forecastTitle = "3. Hourly Trend (24h)"
			unitStr := "°C"
			if m.UseFahrenheit {
				unitStr = "°F"
			}

			temps := m.Weather.Current.HourlyTemp
			displayTemps := make([]float64, len(temps))
			copy(displayTemps, temps)

			if m.UseFahrenheit {
				for i, v := range displayTemps {
					displayTemps[i] = CelsiusToFahrenheit(v)
				}
			}

			minVal, maxVal := 0.0, 0.0
			if len(displayTemps) > 0 {
				minVal, maxVal = displayTemps[0], displayTemps[0]
				for _, v := range displayTemps {
					if v < minVal {
						minVal = v
					}
					if v > maxVal {
						maxVal = v
					}
				}
			}

			sparkline := RenderSparkline(displayTemps)
			sparklineStyled := lipgloss.NewStyle().Foreground(Cyan).Bold(true).Render(sparkline)

			var hourlyContent strings.Builder
			hourlyContent.WriteString(fmt.Sprintf("24-Hour Thermal Curve (%s): Min: %.1f%s │ Max: %.1f%s\n", unitStr, minVal, unitStr, maxVal, unitStr))
			hourlyContent.WriteString(fmt.Sprintf("Curve: %s\n\n", sparklineStyled))
			hourlyContent.WriteString(lipgloss.NewStyle().Foreground(LightGray).Render(" TIME  │ TEMP") + "\n")
			hourlyContent.WriteString(lipgloss.NewStyle().Foreground(Gray).Render(strings.Repeat("─", MaxInt(10, rightWidth-4))) + "\n")

			maxVisibleRows := availableHeight - 8
			if maxVisibleRows < 1 {
				maxVisibleRows = 1
			}

			for i := 0; i < len(displayTemps) && i < maxVisibleRows; i++ {
				timeStr := fmt.Sprintf("%02d:00", i)
				if i < len(m.Weather.Current.HourlyTime) && len(m.Weather.Current.HourlyTime[i]) >= 16 {
					timeStr = m.Weather.Current.HourlyTime[i][11:16]
				}

				tStyle := GetTemperatureStyle(temps[i])
				renderedVal := tStyle.Render(fmt.Sprintf("%.1f %s", displayTemps[i], unitStr))
				hourlyContent.WriteString(fmt.Sprintf(" %-5s │ %s\n", timeStr, renderedVal))
			}

			forecastInnerContent = hourlyContent.String()
	} else {
		forecastTitle = "3. 14-Day Forecast"

			unitHeader := "MAX (°C) │ MIN (°C)"
			if m.UseFahrenheit {
				unitHeader = "MAX (°F) │ MIN (°F)"
			}

			tableHeader := lipgloss.NewStyle().Foreground(LightGray).Render(fmt.Sprintf(" DATE       │ %s │ PRECIPITATION GRAPH", unitHeader)) + "\n"
			tableDivider := lipgloss.NewStyle().Foreground(Gray).Render(strings.Repeat("─", MaxInt(10, rightWidth-4))) + "\n"

			var tableRows strings.Builder
			if m.HasData {
				const nonBarWidth = 42
				barLength := rightWidth - nonBarWidth
				if barLength < 6 {
					barLength = 6
				}

				for i, day := range m.Weather.Daily {
					filledBlocks := (day.PrecipProbability * barLength) / 100
					if filledBlocks > barLength {
						filledBlocks = barLength
					}

					var barStyle lipgloss.Style
					if day.PrecipProbability > 70 {
						barStyle = lipgloss.NewStyle().Foreground(Red)
					} else if day.PrecipProbability > 30 {
						barStyle = lipgloss.NewStyle().Foreground(Yellow)
					} else {
						barStyle = lipgloss.NewStyle().Foreground(Gray)
					}

					emptyStyle := lipgloss.NewStyle().Foreground(BorderGray)

					filledBar := barStyle.Render(strings.Repeat("█", filledBlocks))
					emptyBar := emptyStyle.Render(strings.Repeat("░", barLength-filledBlocks))

					precipStr := fmt.Sprintf("[%s%s] %3d%%", filledBar, emptyBar, day.PrecipProbability)

					maxTemp := day.MaxTemp
					minTemp := day.MinTemp
					if m.UseFahrenheit {
						maxTemp = CelsiusToFahrenheit(maxTemp)
						minTemp = CelsiusToFahrenheit(minTemp)
					}

					maxStyle := GetTemperatureStyle(day.MaxTemp)
					minStyle := GetTemperatureStyle(day.MinTemp)

					renderedMax := maxStyle.Render(fmt.Sprintf("%-6.1f", maxTemp))
					renderedMin := minStyle.Render(fmt.Sprintf("%-6.1f", minTemp))

					rowContent := fmt.Sprintf("  %-10s │ %s │ %s │ %s", day.Date, renderedMax, renderedMin, precipStr)

					if m.ActivePanel == ForecastPanel && i == m.SelectedRow {
						rowContent = fmt.Sprintf("▌ %-10s │ %s │ %s │ %s", day.Date, renderedMax, renderedMin, precipStr)
						tableRows.WriteString(SelectedRowStyle.Render(rowContent) + "\n")
					} else {
						tableRows.WriteString(rowContent + "\n")
					}
				}

				contentLines := strings.Split(strings.TrimSuffix(tableRows.String(), "\n"), "\n")
				maxAllowedRows := availableHeight - 4
				if len(contentLines) > maxAllowedRows && maxAllowedRows > 0 {
					contentLines = contentLines[:maxAllowedRows]
				}

				forecastInnerContent = tableHeader + tableDivider + strings.Join(contentLines, "\n")
			} else {
				statusMsg := "[WAIT] Awaiting telemetry input..."
				if m.Loading {
					statusMsg = fmt.Sprintf("%s Streaming telemetry from Open-Meteo...", m.Spinner.View())
				}
				forecastInnerContent = tableHeader + tableDivider + "\n\n" + statusMsg
			}
	}

	forecastBox := RenderPanelBox(forecastTitle, forecastInnerContent, rightWidth, availableHeight, m.ActivePanel == ForecastPanel, false)

		mainDashboard := lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, forecastBox)

		if m.ShowHelp {
			helpModal := m.renderHelpOverlay()
			mainDashboard = lipgloss.Place(
				m.TermWidth,
				m.TermHeight-1,
				lipgloss.Center,
				lipgloss.Center,
				helpModal,
				lipgloss.WithWhitespaceChars(" "),
			)
		}

		// 5. FOOTER STATUS BAR
		navKeys := KeyStyle.Render("Tab / 1,2,3") + DescStyle.Render("Switch Panel")

		if m.ActivePanel == HistoryPanel {
			navKeys += KeyStyle.Render("j/k") + DescStyle.Render("Nav") +
			KeyStyle.Render("p") + DescStyle.Render("Fav") +
			KeyStyle.Render("Enter") + DescStyle.Render("Load")
		} else if m.ActivePanel == ForecastPanel {
			navKeys += KeyStyle.Render("v") + DescStyle.Render("Hourly") +
			KeyStyle.Render("u") + DescStyle.Render("°C/°F") +
			KeyStyle.Render("p") + DescStyle.Render("Fav") +
			KeyStyle.Render("r") + DescStyle.Render("Refresh") +
			KeyStyle.Render("j/k") + DescStyle.Render("Nav")
		}

		var statusElements []string
		if m.ActivePanel == SearchPanel {
			statusElements = append(statusElements, KeyStyle.Render("Enter"), DescStyle.Render("Search"))
		}
		statusElements = append(statusElements, navKeys)

		if m.ActivePanel != SearchPanel {
			statusElements = append(statusElements, KeyStyle.Render("?"), DescStyle.Render("Help"))
		}

		if m.ActivePanel == SearchPanel {
			statusElements = append(statusElements, KeyStyle.Render("Esc"), DescStyle.Render("Unfocus"))
			statusElements = append(statusElements, KeyStyle.Render("Ctrl+C"), DescStyle.Render("Exit"))
		} else {
			statusElements = append(statusElements, KeyStyle.Render("Esc / Ctrl+C"), DescStyle.Render("Exit"))
		}

		statusElements = append(statusElements,
					lipgloss.NewStyle().Foreground(Gray).Padding(0, 1).Render("│"),
					lipgloss.NewStyle().Foreground(Cyan).Italic(true).Render(fmt.Sprintf("WeatherTUI [%dx%d]", m.TermWidth, m.TermHeight)),
		)

		statusBar := lipgloss.JoinHorizontal(lipgloss.Left, statusElements...)

		return lipgloss.JoinVertical(lipgloss.Left, mainDashboard, statusBar)
}
