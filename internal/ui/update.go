package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles incoming messages and updates state accordingly.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
		case tea.WindowSizeMsg:
			m.TermWidth = msg.Width
			m.TermHeight = msg.Height
			return m, nil

		case WeatherMsg:
			m.Loading = false
			m.HasData = true

			resolvedName := msg.Data
			if resolvedName == "" {
				resolvedName = msg.W.CityName
			}

			m.CityName = resolvedName
			m.LastQuery = resolvedName
			m.Weather = msg.W
			m.SelectedRow = 0

			m.AddRecentLocation(resolvedName)

			m.TextInput.SetValue("")
			return m, nil

		case ErrMsg:
			m.Loading = false
			m.HasData = false
			m.Err = msg
			return m, nil

		case spinner.TickMsg:
			var cmd tea.Cmd
			m.Spinner, cmd = m.Spinner.Update(msg)
			if m.Loading {
				cmds = append(cmds, cmd)
			}

		case tea.KeyMsg:
			// Global help toggle when not focused on search input or when help overlay is already visible
			if msg.String() == "?" && (m.ActivePanel != SearchPanel || m.ShowHelp) {
				m.ShowHelp = !m.ShowHelp
				return m, nil
			}

			if m.ShowHelp {
				switch msg.Type {
					case tea.KeyEsc, tea.KeyCtrlC:
						m.ShowHelp = false
						return m, nil
					case tea.KeyRunes:
						if msg.String() == "q" || msg.String() == "Q" {
							m.ShowHelp = false
							return m, nil
						}
				}
				return m, nil
			}

			// Handle direct numeric shortcuts 1, 2, 3 when SearchPanel is unfocused
			if m.ActivePanel != SearchPanel && msg.Type == tea.KeyRunes {
				switch msg.String() {
					case "1":
						m.ActivePanel = SearchPanel
						m.TextInput.Focus()
						return m, nil
					case "2":
						m.ActivePanel = HistoryPanel
						return m, nil
					case "3":
						m.ActivePanel = ForecastPanel
						return m, nil
				}
			}

			switch msg.Type {
				case tea.KeyCtrlC:
					return m, tea.Quit

				case tea.KeyEsc:
					if m.ActivePanel == SearchPanel {
						m.ActivePanel = ForecastPanel
						m.TextInput.Blur()
						return m, nil
					}
					return m, tea.Quit

				case tea.KeyTab:
					if m.ActivePanel == SearchPanel {
						m.ActivePanel = HistoryPanel
						m.TextInput.Blur()
					} else if m.ActivePanel == HistoryPanel {
						m.ActivePanel = ForecastPanel
					} else {
						m.ActivePanel = SearchPanel
						m.TextInput.Focus()
					}
					return m, nil

				case tea.KeyEnter:
					if m.ActivePanel == SearchPanel {
						city := strings.TrimSpace(m.TextInput.Value())
						if city == "" {
							return m, nil
						}
						m.Loading = true
						m.Err = nil
						m.LastQuery = city
						return m, m.FetchWeatherCmd(city, false)
					}

					if m.ActivePanel == HistoryPanel {
						historyItems := m.GetHistoryItems()
						if len(historyItems) > 0 && m.HistorySelectedRow < len(historyItems) {
							selectedCity := historyItems[m.HistorySelectedRow]
							m.Loading = true
							m.Err = nil
							m.LastQuery = selectedCity
							return m, m.FetchWeatherCmd(selectedCity, false)
						}
					}

				case tea.KeyUp, tea.KeyDown, tea.KeyRunes:
					key := msg.String()

					if m.ActivePanel == HistoryPanel {
						historyItems := m.GetHistoryItems()
						if len(historyItems) > 0 {
							switch key {
								case "k", "up":
									if m.HistorySelectedRow > 0 {
										m.HistorySelectedRow--
									}
									return m, nil
								case "j", "down":
									if m.HistorySelectedRow < len(historyItems)-1 {
										m.HistorySelectedRow++
									}
									return m, nil
								case "p", "P":
									selectedCity := historyItems[m.HistorySelectedRow]
									m.ToggleFavorite(selectedCity)
									if m.HistorySelectedRow >= len(m.GetHistoryItems()) {
										m.HistorySelectedRow = MaxInt(0, len(m.GetHistoryItems())-1)
									}
									return m, nil
							}
						}
					}

					if m.ActivePanel == ForecastPanel {
						if m.HasData {
							switch key {
								case "k", "up":
									if m.SelectedRow > 0 {
										m.SelectedRow--
									}
									return m, nil
								case "j", "down":
									if m.SelectedRow < len(m.Weather.Daily)-1 {
										m.SelectedRow++
									}
									return m, nil
								case "p", "P":
									targetCity := m.CityName
									if targetCity == "" {
										targetCity = m.LastQuery
									}
									if targetCity != "" {
										m.ToggleFavorite(targetCity)
									}
									return m, nil
								case "v", "V":
									m.ShowHourly = !m.ShowHourly
									return m, nil
							}
						}

						if key == "u" || key == "U" {
							m.UseFahrenheit = !m.UseFahrenheit
							m.SaveConfig()
							return m, nil
						}

						if (key == "r" || key == "R") && m.LastQuery != "" {
							m.Loading = true
							m.Err = nil
							return m, m.FetchWeatherCmd(m.LastQuery, true)
						}
					}
			}
	}

	if m.ActivePanel == SearchPanel && !m.ShowHelp {
		var inputCmd tea.Cmd
		m.TextInput, inputCmd = m.TextInput.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}
