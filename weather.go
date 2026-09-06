package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// DailyForecast holds daily metric values.
type DailyForecast struct {
	Date              string
	MaxTemp           float64
	MinTemp           float64
	PrecipProbability int
}

// WeatherData holds atmospheric current metrics and metadata.
type WeatherData struct {
	Temperature       float64
	WindSpeed         float64
	UvIndex           float64
	PrecipProbability int
	Humidity          int
	Sunrise           string
	Sunset            string
	AirQualityIndex   int
}

// WeatherResponse aggregates current telemetry, forecast cycles, target location name, and cache timestamp.
type WeatherResponse struct {
	CityName  string
	Current   WeatherData
	Daily     []DailyForecast
	Timestamp int64
}

type geoAPIResponse struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Country   string  `json:"country"`
	} `json:"results"`
}

type weatherAPIResponse struct {
	Current struct {
		Temperature float64 `json:"temperature_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		UvIndex     float64 `json:"uv_index"`
		Precip      float64 `json:"precipitation_probability"`
		Humidity    float64 `json:"relative_humidity_2m"`
	} `json:"current"`
	Daily struct {
		Time              []string  `json:"time"`
		TempMax           []float64 `json:"temperature_2m_max"`
		TempMin           []float64 `json:"temperature_2m_min"`
		PrecipProbability []float64 `json:"precipitation_probability_max"`
		Sunrise           []string  `json:"sunrise"`
		Sunset            []string  `json:"sunset"`
	} `json:"daily"`
}

type airQualityAPIResponse struct {
	Current struct {
		EuropeanAQI float64 `json:"european_aqi"`
	} `json:"current"`
}

// WeatherClient orchestrates HTTP calls against Open-Meteo APIs.
type WeatherClient struct {
	cache  *CacheService
	client *http.Client
}

// NewWeatherClient initializes client instance with standard 10s timeout boundary.
func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		cache:  NewCacheService(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// getSystemLanguage attempts to detect the system's locale language code.
// It parses common POSIX environment variables and defaults to "en" if missing.
func getSystemLanguage() string {
	envVars := []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"}
	for _, env := range envVars {
		val := os.Getenv(env)
		if val != "" && val != "C" {
			// Parse formats like "it_IT.UTF-8" or "en_US"
			parts := strings.Split(val, "_")
			if len(parts) > 0 && len(parts[0]) == 2 {
				return strings.ToLower(parts[0])
			}
		}
	}
	return "en" // Fallback language
}

// GetWeather queries Open-Meteo endpoints or returns cached responses.
func (wc *WeatherClient) GetWeather(city string) (WeatherResponse, string, error) {
	if cachedResponse, found := wc.cache.Get(city); found {
		slog.Debug("Cache HIT", "city", city, "resolved_name", cachedResponse.CityName)
		return cachedResponse, cachedResponse.CityName, nil
	}

	sysLang := getSystemLanguage()
	slog.Info("Cache MISS. Executing geocoding lookup", "city", city, "lang", sysLang)

	encodedCity := url.QueryEscape(city)
	geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=%s&format=json", encodedCity, sysLang)

	resp, err := wc.client.Get(geoURL)
	if err != nil {
		slog.Error("Geocoding network error", "city", city, "error", err)
		return WeatherResponse{}, "", fmt.Errorf("geocoding network error: %w", err)
	}
	defer resp.Body.Close()

	var geoData geoAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil || len(geoData.Results) == 0 {
		slog.Error("Location target unresolved", "city", city)
		return WeatherResponse{}, "", fmt.Errorf("location target not found")
	}
	loc := geoData.Results[0]
	fullName := fmt.Sprintf("%s (%s)", loc.Name, loc.Country)

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
		"&current=temperature_2m,wind_speed_10m,uv_index,precipitation_probability,relative_humidity_2m"+
		"&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=14",
		loc.Latitude, loc.Longitude,
	)

		slog.Info("Dispatching forecast request", "url", forecastURL)

		wResp, err := wc.client.Get(forecastURL)
		if err != nil {
			slog.Error("Forecast network failure", "url", forecastURL, "error", err)
			return WeatherResponse{}, "", fmt.Errorf("forecast network error: %w", err)
		}
		defer wResp.Body.Close()

		var apiMeteo weatherAPIResponse
		if err := json.NewDecoder(wResp.Body).Decode(&apiMeteo); err != nil {
			slog.Error("Forecast payload decoding failed", "error", err)
			return WeatherResponse{}, "", fmt.Errorf("forecast decoding error: %w", err)
		}

		// Non-blocking Air Quality Index lookup
		aqiURL := fmt.Sprintf("https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%f&longitude=%f&current=european_aqi", loc.Latitude, loc.Longitude)
		europeanAQI := 0
		if aqiResp, err := wc.client.Get(aqiURL); err == nil {
			var apiAQI airQualityAPIResponse
			if json.NewDecoder(aqiResp.Body).Decode(&apiAQI) == nil {
				europeanAQI = int(apiAQI.Current.EuropeanAQI)
			}
			aqiResp.Body.Close()
		} else {
			slog.Warn("Air Quality API degraded gracefully", "error", err)
		}

		sunriseStr, sunsetStr := "N/A", "N/A"
		if len(apiMeteo.Daily.Sunrise) > 0 && len(apiMeteo.Daily.Sunrise[0]) >= 16 {
			sunriseStr = apiMeteo.Daily.Sunrise[0][11:16]
		}
		if len(apiMeteo.Daily.Sunset) > 0 && len(apiMeteo.Daily.Sunset[0]) >= 16 {
			sunsetStr = apiMeteo.Daily.Sunset[0][11:16]
		}

		currentData := WeatherData{
			Temperature:       apiMeteo.Current.Temperature,
			WindSpeed:         apiMeteo.Current.WindSpeed,
			UvIndex:           apiMeteo.Current.UvIndex,
			PrecipProbability: int(apiMeteo.Current.Precip),
			Humidity:          int(apiMeteo.Current.Humidity),
			Sunrise:           sunriseStr,
			Sunset:            sunsetStr,
			AirQualityIndex:   europeanAQI,
		}

		dailyCount := len(apiMeteo.Daily.Time)
		dailyForecasts := make([]DailyForecast, 0, dailyCount)
		for i := 0; i < dailyCount; i++ {
			maxTemp := 0.0
			if i < len(apiMeteo.Daily.TempMax) {
				maxTemp = apiMeteo.Daily.TempMax[i]
			}
			minTemp := 0.0
			if i < len(apiMeteo.Daily.TempMin) {
				minTemp = apiMeteo.Daily.TempMin[i]
			}
			precip := 0
			if i < len(apiMeteo.Daily.PrecipProbability) {
				precip = int(apiMeteo.Daily.PrecipProbability[i])
			}

			dailyForecasts = append(dailyForecasts, DailyForecast{
				Date:              apiMeteo.Daily.Time[i],
				MaxTemp:           maxTemp,
				MinTemp:           minTemp,
				PrecipProbability: precip,
			})
		}

		finalResponse := WeatherResponse{
			CityName:  fullName,
			Current:   currentData,
			Daily:     dailyForecasts,
			Timestamp: time.Now().UnixMilli(),
		}

		wc.cache.Put(city, finalResponse)
		return finalResponse, fullName, nil
}
