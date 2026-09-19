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

	sb.WriteString(HelpSectionStyle.Render("RECENT & FAVORITE LOCATIONS PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("j / k"), DescStyle.Render("Navigate items")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Enter"), DescStyle.Render("Load selected location")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("p"), DescStyle.Render("Toggle favorite status")))

	sb.WriteString(HelpSectionStyle.Render("FORECAST PANEL") + "\n")
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("j / k"), DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("Up / Down"), DescStyle.Render("Navigate 14-day daily records")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("u"), DescStyle.Render("Toggle temperature unit (°C / °F)")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n", KeyStyle.Render("p"), DescStyle.Render("Toggle active city favorite status")))
	sb.WriteString(fmt.Sprintf("  %-12s %s\n\n", KeyStyle.Render("r"), DescStyle.Render("Force refresh data (bypass cache)")))

	sb.WriteString(lipgloss.NewStyle().Foreground(Gray).Italic(true).Render("Press ? or Esc to return to dashboard"))

	return HelpOverlayStyle.Render(sb.String())
}

// View renders the application user interface.
func (m Model) View() string {
	if m.TermWidth < 80 || m.TermHeight < 20 {
		return SmallScreenStyle.
		Width(m.TermWidth).
		Height(m.TermHeight).
		Render("Terminal screen too small. Please resize.")
	}

	availableHeight := m.TermHeight - 4
	if availableHeight < 16 {
		availableHeight = 16
	}

	leftWidth := int(float64(m.TermWidth) * 0.35)
	if leftWidth < 40 {
		leftWidth = 40
	}
	rightWidth := m.TermWidth - leftWidth - 2
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
		searchTitle := " [1] SEARCH ENGINE "
		searchBoxStyleToUse := BoxStyle
		if m.ActivePanel == SearchPanel {
			searchBoxStyleToUse = ActiveBoxStyle
		}

		searchContent := fmt.Sprintf("%s\n\n%s", PanelTitleStyle.Render(searchTitle), m.TextInput.View())
		searchBox := searchBoxStyleToUse.
		Width(leftWidth - 2).
		Height(searchInnerHeight).
		MaxWidth(leftWidth - 2).
		MaxHeight(searchInnerHeight + 2).
		Render(searchContent)

		// 2. RECENT & FAVORITE LOCATIONS PANEL
		historyTitle := " [2] RECENT & FAVORITES "
		historyBoxStyleToUse := BoxStyle
		if m.ActivePanel == HistoryPanel {
			historyBoxStyleToUse = ActiveBoxStyle
		}

		historyHeader := PanelTitleStyle.Render(historyTitle)
		var historyContent strings.Builder
		historyContent.WriteString(historyHeader + "\n\n")

		historyItems := m.GetHistoryItems()
		if len(historyItems) == 0 {
			historyContent.WriteString(lipgloss.NewStyle().Foreground(Gray).Render("No recent lookups"))
		} else {
			maxVisibleRows := historyInnerHeight - 2
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

				displayStr := loc
				if isFav {
					displayStr = "[FAV] " + loc
				} else {
					displayStr = "      " + loc
				}

				rowStr := fmt.Sprintf(" %-30s", displayStr)
				if len(rowStr) > leftWidth-6 {
					rowStr = rowStr[:leftWidth-6]
				}

				if m.ActivePanel == HistoryPanel && i == m.HistorySelectedRow {
					historyContent.WriteString(SelectedRowStyle.Render(rowStr) + "\n")
				} else if isFav {
					historyContent.WriteString(FavoriteStyle.Render(rowStr) + "\n")
				} else {
					historyContent.WriteString(ValueStyle.Render(rowStr) + "\n")
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
		displayName := m.CityName
		if displayName == "" && m.HasData {
			displayName = m.Weather.CityName
		}

		if m.HasData && displayName != "" {
			cacheBadge := ""
			if m.Weather.FromCache {
				cacheBadge = lipgloss.NewStyle().Foreground(Yellow).Bold(true).Render(" [CACHE HIT]")
			}
			favBadge := ""
			if strings.EqualFold(displayName, m.FavoriteCity) || strings.EqualFold(m.LastQuery, m.FavoriteCity) {
				favBadge = FavoriteStyle.Render(" [FAV]")
			}
			currentHeaderTitle = fmt.Sprintf(" MONITOR: %s%s%s ", displayName, favBadge, cacheBadge)
		}
		currentHeader := PanelTitleStyle.Render(currentHeaderTitle)

		currentStyle := BoxStyle
		if m.Err != nil {
			currentStyle = ErrorBoxStyle
		}

		if m.Loading {
			loadingText := fmt.Sprintf("%s Fetching telemetry pipeline...", m.Spinner.View())
			placeholderText := fmt.Sprintf("STATUS: PARSING METRICS...\n\n%s\nSynchronizing Open-Meteo DB\nStream pipelines active...", loadingText)
			currentBox = currentStyle.
			Width(leftWidth - 2).
			Height(currentInnerHeight).
			MaxWidth(leftWidth - 2).
			MaxHeight(currentInnerHeight + 2).
			Render(currentHeader + "\n\n" + placeholderText)
		} else if m.HasData {
			aqiVal := m.Weather.Current.AirQualityIndex
			var aqiDesc string
			if aqiVal <= 20 {
				aqiDesc = lipgloss.NewStyle().Foreground(Green).Render(fmt.Sprintf("%d (EXCELLENT)", aqiVal))
			} else if aqiVal <= 40 {
				aqiDesc = lipgloss.NewStyle().Foreground(Yellow).Render(fmt.Sprintf("%d (POOR)", aqiVal))
			} else {
				aqiDesc = lipgloss.NewStyle().Foreground(Red).Render(fmt.Sprintf("%d (CRITICAL)", aqiVal))
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

			metricsContent := fmt.Sprintf(
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s\n"+
				"%s  %-12s %s",
				 LabelStyle.Render("[TEMP]"), "Temperature:", renderedTemp,
						      LabelStyle.Render("[HUMI]"), "Humidity:", renderedHumidity,
						      LabelStyle.Render("[WIND]"), "Wind:", renderedWind,
						      LabelStyle.Render("[UVIN]"), "UV Index:", uvDesc,
						      LabelStyle.Render("[SUNR]"), "Sun Rise:", lipgloss.NewStyle().Foreground(Blue).Render(m.Weather.Current.Sunrise),
						      LabelStyle.Render("[SUNS]"), "Sun Set:", lipgloss.NewStyle().Foreground(Blue).Render(m.Weather.Current.Sunset),
						      LabelStyle.Render("[AQI ]"), "Air Quality:", aqiDesc,
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
			if m.Err != nil {
				placeholderText = lipgloss.NewStyle().Foreground(Red).Render(fmt.Sprintf("[ERR] SYSTEM EXCEPTION:\n%v", m.Err))
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
		forecastHeader := PanelTitleStyle.Render(" [3] FORECAST (14-DAY CORE) ") + "\n\n"

			forecastBoxStyleToUse := BoxStyle
				if m.ActivePanel == ForecastPanel {
					forecastBoxStyleToUse = ActiveBoxStyle
				}

				const fixedTableColumnsWidth = 36
				dynamicBarLength := rightWidth - fixedTableColumnsWidth - 9
				if dynamicBarLength < 10 {
					dynamicBarLength = 10
				}

				graphHeaderPadding := strings.Repeat(" ", MaxInt(0, dynamicBarLength-15))
				unitHeader := "MAX (°C)│ MIN (°C)"
				if m.UseFahrenheit {
					unitHeader = "MAX (°F)│ MIN (°F)"
				}
				tableHeader := lipgloss.NewStyle().Foreground(LightGray).Render(fmt.Sprintf(" DATE       │ %s │ PRECIPITATION GRAPH%s", unitHeader, graphHeaderPadding)) + "\n"
				tableDivider := lipgloss.NewStyle().Foreground(Gray).Render(strings.Repeat("─", MaxInt(10, rightWidth-4))) + "\n"

				var tableRows strings.Builder
				if m.HasData {
					for i, day := range m.Weather.Daily {
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
							barStyle = lipgloss.NewStyle().Foreground(Red)
						} else if day.PrecipProbability > 30 {
							barStyle = lipgloss.NewStyle().Foreground(Yellow)
						} else {
							barStyle = lipgloss.NewStyle().Foreground(Gray)
						}

						renderedBar := barStyle.Render(fmt.Sprintf("%s %3d%%", barStr.String(), day.PrecipProbability))

						maxTemp := day.MaxTemp
						minTemp := day.MinTemp
						if m.UseFahrenheit {
							maxTemp = CelsiusToFahrenheit(maxTemp)
							minTemp = CelsiusToFahrenheit(minTemp)
						}

						maxStyle := GetTemperatureStyle(day.MaxTemp)
						minStyle := GetTemperatureStyle(day.MinTemp)

						renderedMax := maxStyle.Render(fmt.Sprintf("%-7.1f", maxTemp))
						renderedMin := minStyle.Render(fmt.Sprintf("%-7.1f", minTemp))

						row := fmt.Sprintf(" %-10s │  %s │  %s │ %s", day.Date, renderedMax, renderedMin, renderedBar)

						if m.ActivePanel == ForecastPanel && i == m.SelectedRow {
							row = SelectedRowStyle.Render(row)
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
					if m.Loading {
						statusMsg = fmt.Sprintf("%s Streaming telemetry from Open-Meteo clusters...", m.Spinner.View())
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

				if m.ShowHelp {
					helpModal := m.renderHelpOverlay()
					mainDashboard = lipgloss.Place(
						m.TermWidth,
				    m.TermHeight-2,
				    lipgloss.Center,
				    lipgloss.Center,
				    helpModal,
				    lipgloss.WithWhitespaceChars(" "),
					)
				}

				// 5. FOOTER STATUS BAR
				navKeys := KeyStyle.Render("Tab / 1,2,3") + DescStyle.Render("Switch View")

				if m.ActivePanel == HistoryPanel {
					navKeys += KeyStyle.Render("j/k") + DescStyle.Render("Navigate") +
					KeyStyle.Render("p") + DescStyle.Render("Favorite") +
					KeyStyle.Render("Enter") + DescStyle.Render("Load")
				} else if m.ActivePanel == ForecastPanel {
					navKeys += KeyStyle.Render("u") + DescStyle.Render("Toggle °C/°F") +
					KeyStyle.Render("p") + DescStyle.Render("Favorite") +
					KeyStyle.Render("r") + DescStyle.Render("Refresh") +
					KeyStyle.Render("j/k") + DescStyle.Render("Rows")
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
							lipgloss.NewStyle().Foreground(Cyan).Italic(true).Render(fmt.Sprintf("WeatherTUI - [res: %dx%d]", m.TermWidth, m.TermHeight)),
				)

				statusBar := lipgloss.JoinHorizontal(lipgloss.Left, statusElements...)

				fullLayout := lipgloss.JoinVertical(lipgloss.Left, mainDashboard, statusBar)

				return lipgloss.NewStyle().
				MaxWidth(m.TermWidth).
				MaxHeight(m.TermHeight).
				Render("\n" + fullLayout + "\n")
}
