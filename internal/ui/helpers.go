package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// CelsiusToFahrenheit converts degrees Celsius to Fahrenheit.
func CelsiusToFahrenheit(c float64) float64 {
	return (c * 9 / 5) + 32
}

// DegreesToCompass converts wind direction degrees to an ASCII direction arrow and compass point label.
func DegreesToCompass(degrees float64) string {
	arrows := []string{"↓", "↙", "←", "↖", "↑", "↗", "→", "↘"}
	directions := []string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}

	arrowIdx := int((degrees + 22.5) / 45.0) % 8
	dirIdx := int((degrees + 11.25) / 22.5) % 16

	return fmt.Sprintf("%s %s", arrows[arrowIdx], directions[dirIdx])
}

// GetTemperatureStyle returns a Lipgloss style dynamically based on temperature threshold values.
func GetTemperatureStyle(tempCelsius float64) lipgloss.Style {
	baseStyle := lipgloss.NewStyle().Bold(true)
	if tempCelsius < 0 {
		return baseStyle.Foreground(Blue)
	} else if tempCelsius <= 15 {
		return baseStyle.Foreground(Cyan)
	} else if tempCelsius <= 25 {
		return baseStyle.Foreground(Green)
	} else if tempCelsius < 30 {
		return baseStyle.Foreground(Orange)
	}
	return baseStyle.Foreground(Red)
}

// GetHumidityStyle returns a Lipgloss style dynamically based on relative humidity percentage.
func GetHumidityStyle(humidity int) lipgloss.Style {
	baseStyle := lipgloss.NewStyle().Bold(true)
	if humidity <= 30 {
		return baseStyle.Foreground(Green)
	} else if humidity <= 50 {
		return baseStyle.Foreground(Yellow)
	} else if humidity <= 75 {
		return baseStyle.Foreground(Orange)
	}
	return baseStyle.Foreground(Red)
}

// GetWindSpeedStyle returns a Lipgloss style dynamically based on wind speed in km/h.
func GetWindSpeedStyle(speedKmH float64) lipgloss.Style {
	baseStyle := lipgloss.NewStyle().Bold(true)
	if speedKmH <= 15.0 {
		return baseStyle.Foreground(Green)
	} else if speedKmH <= 30.0 {
		return baseStyle.Foreground(Yellow)
	} else if speedKmH <= 50.0 {
		return baseStyle.Foreground(Orange)
	}
	return baseStyle.Foreground(Red)
}

// GetUVIndexDesc returns a formatted string with warning color styling depending on the UV index value.
func GetUVIndexDesc(uv float64) string {
	if uv <= 2 {
		return lipgloss.NewStyle().Foreground(Green).Render(fmt.Sprintf("%.1f (LOW)", uv))
	} else if uv <= 5 {
		return lipgloss.NewStyle().Foreground(Yellow).Render(fmt.Sprintf("%.1f (MODERATE)", uv))
	} else if uv <= 7 {
		return lipgloss.NewStyle().Foreground(Yellow).Bold(true).Render(fmt.Sprintf("%.1f (HIGH)", uv))
	} else if uv <= 10 {
		return lipgloss.NewStyle().Foreground(Red).Render(fmt.Sprintf("%.1f (VERY HIGH)", uv))
	}
	return lipgloss.NewStyle().Foreground(Red).Bold(true).Render(fmt.Sprintf("%.1f (EXTREME)", uv))
}

// MaxInt returns the larger of two integer values.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
