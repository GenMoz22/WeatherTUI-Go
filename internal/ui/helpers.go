package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// CalculateApparentTemperature calculates perceived temperature (°C) based on actual temperature (°C),
// relative humidity (%), and wind speed (km/h) using the Australian National Weather Service / Steadman formula.
func CalculateApparentTemperature(tempCelsius float64, humidity int, windSpeedKmH float64) float64 {
	// Convert wind speed from km/h to m/s as required by the Steadman apparent temperature model
	windSpeedMS := windSpeedKmH / 3.6

	// Calculate water vapor pressure (hPa) from temperature and relative humidity
	humidityRatio := float64(humidity) / 100.0
	e := humidityRatio * 6.105 * math.Exp((17.27*tempCelsius)/(237.7+tempCelsius))

	// Australian Apparent Temperature (AT) formula: AT = T + 0.33*e - 0.70*ws - 4.00
	apparentTemp := tempCelsius + (0.33 * e) - (0.70 * windSpeedMS) - 4.00

	return apparentTemp
}

// CelsiusToFahrenheit converts degrees Celsius to Fahrenheit.
func CelsiusToFahrenheit(c float64) float64 {
	return (c * 9 / 5) + 32
}

// KmHToMph converts wind speed from km/h to mph.
func KmHToMph(kmh float64) float64 {
	return kmh * 0.621371
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

// GetUVIndexDesc renders the UV index with a block-style progress bar and severity label.
func GetUVIndexDesc(uv float64) string {
	const totalBlocks = 10
	cappedUV := uv
	if cappedUV > 11.0 {
		cappedUV = 11.0
	}
	if cappedUV < 0 {
		cappedUV = 0
	}

	filledBlocks := int((cappedUV / 11.0) * float64(totalBlocks))
	if filledBlocks > totalBlocks {
		filledBlocks = totalBlocks
	}

	var barBuilder strings.Builder
	barBuilder.WriteString("[")
	for i := 0; i < totalBlocks; i++ {
		if i < filledBlocks {
			barBuilder.WriteString("█")
		} else {
			barBuilder.WriteString("░")
		}
	}
	barBuilder.WriteString("]")

	var label string
	var style lipgloss.Style

	if uv <= 2 {
		label = "LOW"
		style = lipgloss.NewStyle().Foreground(Green).Bold(true)
	} else if uv <= 5 {
		label = "MODERATE"
		style = lipgloss.NewStyle().Foreground(Yellow).Bold(true)
	} else if uv <= 7 {
		label = "HIGH"
		style = lipgloss.NewStyle().Foreground(Yellow).Bold(true)
	} else if uv <= 10 {
		label = "VERY HIGH"
		style = lipgloss.NewStyle().Foreground(Red).Bold(true)
	} else {
		label = "EXTREME"
		style = lipgloss.NewStyle().Foreground(Red).Bold(true)
	}

	return style.Render(fmt.Sprintf("%s %.1f (%s)", barBuilder.String(), uv, label))
}

// RenderSparkline converts a float slice into a Braille/Sparkline character sequence.
func RenderSparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}

	sparklines := []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	minVal := values[0]
	maxVal := values[0]

	for _, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	valRange := maxVal - minVal

	var sb strings.Builder
	for _, v := range values {
		var idx int
		if valRange == 0 {
			idx = 3
		} else {
			normalized := (v - minVal) / valRange
			idx = int(math.Floor(normalized * float64(len(sparklines)-1)))
			if idx >= len(sparklines) {
				idx = len(sparklines) - 1
			}
			if idx < 0 {
				idx = 0
			}
		}
		sb.WriteRune(sparklines[idx])
	}

	return sb.String()
}

// MaxInt returns the larger of two integer values.
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
