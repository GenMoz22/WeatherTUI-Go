package weather

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var coordRegex = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*[\s,]\s*(-?\d+(?:\.\d+)?)\s*$`)

// MoonPhaseData represents moon illumination percentage and its icon representation.
type MoonPhaseData struct {
	PhaseIcon          string
	IlluminationPercent int
}

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
	WindDirection     float64
	UvIndex           float64
	PrecipProbability int
	Humidity          int
	Sunrise           string
	Sunset            string
	AirQualityIndex   int
	MoonPhase         MoonPhaseData
}

// WeatherResponse aggregates telemetry, forecast cycles, location name, and cache timestamp.
type WeatherResponse struct {
	CityName  string
	Current   WeatherData
	Daily     []DailyForecast
	Timestamp int64
	FromCache bool
}

// CacheService provides concurrent-safe in-memory caching for weather telemetry.
type CacheService struct {
	mu                sync.RWMutex
	cache             map[string]WeatherResponse
	expirationMinutes int64
}

// NewCacheService initializes the cache layer with a default 30-minute expiration TTL.
func NewCacheService() *CacheService {
	return &CacheService{
		cache:             make(map[string]WeatherResponse),
		expirationMinutes: 30,
	}
}

func (cs *CacheService) isExpired(timestamp int64) bool {
	return (time.Now().UnixMilli() - timestamp) > (cs.expirationMinutes * 60 * 1000)
}

// Get retrieves cached weather telemetry if present and valid.
func (cs *CacheService) Get(city string) (WeatherResponse, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	key := strings.ToLower(strings.TrimSpace(city))
	cached, found := cs.cache[key]
	if found && !cs.isExpired(cached.Timestamp) {
		return cached, true
	}
	return WeatherResponse{}, false
}

// Put inserts weather telemetry into cache.
func (cs *CacheService) Put(city string, response WeatherResponse) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(city))
	cs.cache[key] = response
}

// Delete invalidates a cached entry.
func (cs *CacheService) Delete(city string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	key := strings.ToLower(strings.TrimSpace(city))
	delete(cs.cache, key)
}

type geoAPIResponse struct {
	Results []struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Name      string  `json:"name"`
		Country   string  `json:"country"`
	} `json:"results"`
}

type reverseGeoAPIResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name"`
	Country   string  `json:"country"`
}

type weatherAPIResponse struct {
	Current struct {
		Temperature   float64 `json:"temperature_2m"`
		WindSpeed     float64 `json:"wind_speed_10m"`
		WindDirection float64 `json:"wind_direction_10m"`
		UvIndex       float64 `json:"uv_index"`
		Precip        float64 `json:"precipitation_probability"`
		Humidity      float64 `json:"relative_humidity_2m"`
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

// WeatherClient orchestrates HTTP calls against Open-Meteo services.
type WeatherClient struct {
	cache  *CacheService
	client *http.Client
}

// NewWeatherClient creates a client instance enforcing a 10s timeout limit.
func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		cache:  NewCacheService(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// getSystemLanguage detects system language locale code or defaults to "en".
func getSystemLanguage() string {
	envVars := []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"}
	for _, env := range envVars {
		val := os.Getenv(env)
		if val != "" && val != "C" {
			parts := strings.Split(val, "_")
			if len(parts) > 0 && len(parts[0]) == 2 {
				return strings.ToLower(parts[0])
			}
		}
	}
	return "en"
}

// ParseCoordinates checks if input matches latitude and longitude pattern.
func ParseCoordinates(query string) (float64, float64, bool) {
	matches := coordRegex.FindStringSubmatch(query)
	if len(matches) != 3 {
		return 0, 0, false
	}

	lat, errLat := strconv.ParseFloat(matches[1], 64)
	lon, errLon := strconv.ParseFloat(matches[2], 64)

	if errLat != nil || errLon != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}

	return lat, lon, true
}

// sanitizeCityQuery extracts the clean city name by removing country suffixes in parentheses.
func sanitizeCityQuery(city string) string {
	city = strings.TrimSpace(city)
	if idx := strings.Index(city, " ("); idx != -1 {
		return strings.TrimSpace(city[:idx])
	}
	return city
}

// reverseGeocode attempts to resolve coordinates to a location name using Open-Meteo reverse geocoding API.
func (wc *WeatherClient) reverseGeocode(lat, lon float64, lang string) string {
	reverseURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/get?latitude=%f&longitude=%f&language=%s&format=json", lat, lon, lang)

	resp, err := wc.client.Get(reverseURL)
	if err != nil {
		slog.Warn("Reverse geocoding network error, falling back to coordinates string", "error", err)
		return fmt.Sprintf("%.4f, %.4f", lat, lon)
	}
	defer resp.Body.Close()

	var res reverseGeoAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || res.Name == "" {
		slog.Warn("Reverse geocoding resolution failed, falling back to coordinates string")
		return fmt.Sprintf("%.4f, %.4f", lat, lon)
	}

	if res.Country != "" {
		return fmt.Sprintf("%s (%s)", res.Name, res.Country)
	}
	return res.Name
}

// CalculateMoonPhase computes lunar phase icon and illumination percentage for a target time.
func CalculateMoonPhase(t time.Time) MoonPhaseData {
	year := t.Year()
	month := int(t.Month())
	day := t.Day()

	if month < 3 {
		year--
		month += 12
	}

	c := float64(year) / 100.0
	b := 2.0 - c + math.Floor(c/4.0)
	jd := math.Floor(365.25*float64(year+4716)) + math.Floor(30.6001*float64(month+1)) + float64(day) + b - 1524.5

	synodicMonth := 29.53058867
	knownNewMoonJD := 2451549.5 // Reference New Moon: Jan 6, 2000 18:14 UTC

	daysSinceNew := jd - knownNewMoonJD
	moonAge := math.Mod(daysSinceNew, synodicMonth)
	if moonAge < 0 {
		moonAge += synodicMonth
	}

	// Calculate illumination ratio based on phase angle (0.0 to 1.0)
	phaseAngle := (moonAge / synodicMonth) * 2.0 * math.Pi
	illumination := (1.0 - math.Cos(phaseAngle)) / 2.0
	illuminationPercent := int(math.Round(illumination * 100.0))

	var icon string
	switch {
		case moonAge < 1.84566:
			icon = "🌑" // New Moon
		case moonAge < 5.53699:
			icon = "🌒" // Waxing Crescent
		case moonAge < 9.22831:
			icon = "🌓" // First Quarter
		case moonAge < 12.91963:
			icon = "🌔" // Waxing Gibbous
		case moonAge < 16.61096:
			icon = "🌕" // Full Moon
		case moonAge < 20.30228:
			icon = "🌖" // Waning Gibbous
		case moonAge < 23.99361:
			icon = "🌗" // Last Quarter
		case moonAge < 27.68493:
			icon = "🌘" // Waning Crescent
		default:
			icon = "🌑" // New Moon
	}

	return MoonPhaseData{
		PhaseIcon:          icon,
		IlluminationPercent: illuminationPercent,
	}
}

// GetWeather queries Open-Meteo endpoints or retrieves cached entries.
func (wc *WeatherClient) GetWeather(city string, forceRefresh bool) (WeatherResponse, string, error) {
	cleanQuery := sanitizeCityQuery(city)

	if forceRefresh {
		wc.cache.Delete(city)
		wc.cache.Delete(cleanQuery)
	} else if cachedResponse, found := wc.cache.Get(city); found {
		slog.Debug("Cache HIT", "city", city, "resolved_name", cachedResponse.CityName)
		cachedResponse.FromCache = true
		return cachedResponse, cachedResponse.CityName, nil
	} else if cachedResponse, found := wc.cache.Get(cleanQuery); found {
		slog.Debug("Cache HIT via clean query", "city", cleanQuery, "resolved_name", cachedResponse.CityName)
		cachedResponse.FromCache = true
		return cachedResponse, cachedResponse.CityName, nil
	}

	sysLang := getSystemLanguage()
	var lat, lon float64
	var fullName string

	if parsedLat, parsedLon, isCoords := ParseCoordinates(city); isCoords {
		lat = parsedLat
		lon = parsedLon
		slog.Info("Coordinates pattern matched. Executing reverse geocoding lookup", "lat", lat, "lon", lon)
		fullName = wc.reverseGeocode(lat, lon, sysLang)
	} else {
		slog.Info("Cache MISS or Refresh. Executing geocoding lookup", "city", cleanQuery, "lang", sysLang)

		encodedCity := url.QueryEscape(cleanQuery)
		geoURL := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=%s&format=json", encodedCity, sysLang)

		resp, err := wc.client.Get(geoURL)
		if err != nil {
			slog.Error("Geocoding network error", "city", cleanQuery, "error", err)
			return WeatherResponse{}, "", fmt.Errorf("geocoding network error: %w", err)
		}
		defer resp.Body.Close()

		var geoData geoAPIResponse
		if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil || len(geoData.Results) == 0 {
			slog.Error("Location target unresolved", "city", cleanQuery)
			return WeatherResponse{}, "", fmt.Errorf("location target not found")
		}
		loc := geoData.Results[0]
		lat = loc.Latitude
		lon = loc.Longitude
		if loc.Country != "" {
			fullName = fmt.Sprintf("%s (%s)", loc.Name, loc.Country)
		} else {
			fullName = loc.Name
		}
	}

	forecastURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
		"&current=temperature_2m,wind_speed_10m,wind_direction_10m,uv_index,precipitation_probability,relative_humidity_2m"+
		"&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=14",
		lat, lon,
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

		aqiURL := fmt.Sprintf("https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%f&longitude=%f&current=european_aqi", lat, lon)
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

		moonData := CalculateMoonPhase(time.Now())

		currentData := WeatherData{
			Temperature:       apiMeteo.Current.Temperature,
			WindSpeed:         apiMeteo.Current.WindSpeed,
			WindDirection:     apiMeteo.Current.WindDirection,
			UvIndex:           apiMeteo.Current.UvIndex,
			PrecipProbability: int(apiMeteo.Current.Precip),
			Humidity:          int(apiMeteo.Current.Humidity),
			Sunrise:           sunriseStr,
			Sunset:            sunsetStr,
			AirQualityIndex:   europeanAQI,
			MoonPhase:         moonData,
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
			FromCache: false,
		}

		wc.cache.Put(city, finalResponse)
		wc.cache.Put(cleanQuery, finalResponse)
		wc.cache.Put(fullName, finalResponse)

		return finalResponse, fullName, nil
}
