package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"
)

// Modelli Dati Globali
type DailyForecast struct {
	Date              string
	MaxTemp           float64
	MinTemp           float64
	PrecipProbability int
}

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

type WeatherResponse struct {
	Current   WeatherData
	Daily     []DailyForecast
	Timestamp int64
}

// Strutture di unmarshalling JSON interne alle API
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

// Client API
type WeatherClient struct {
	cache  *CacheService
	client *http.Client
}

func NewWeatherClient() *WeatherClient {
	return &WeatherClient{
		cache:  NewCacheService(),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (wc *WeatherClient) GetWeather(city string) (WeatherResponse, string, error) {
	if cachedResponse, found := wc.cache.Get(city); found {
		slog.Debug("Cache HIT", "city", city)
		return cachedResponse, city, nil
	}

	// 1. Log Cache Miss e inizio Geocoding
	slog.Info("Cache MISS. Esecuzione Geocoding per", "city", city)

	encodedCity := url.QueryEscape(city)
	geoUrl := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=it&format=json", encodedCity)

	resp, err := wc.client.Get(geoUrl)
	if err != nil {
		slog.Error("Errore rete geocoding", "city", city, "error", err)
		return WeatherResponse{}, "", fmt.Errorf("geocoding network error: %w", err)
	}
	defer resp.Body.Close()

	var geoData geoAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil || len(geoData.Results) == 0 {
		slog.Error("Target posizione non trovato", "city", city)
		return WeatherResponse{}, "", fmt.Errorf("location target not found")
	}
	loc := geoData.Results[0]
	fullName := fmt.Sprintf("%s (%s)", loc.Name, loc.Country)

	// 2. Fetch Weather Metrics + Log URL API
	forecastUrl := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%f&longitude=%f"+
		"&current=temperature_2m,wind_speed_10m,uv_index,precipitation_probability,relative_humidity_2m"+
		"&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,sunrise,sunset&timezone=auto&forecast_days=14",
		loc.Latitude, loc.Longitude,
	)

		// Invia il log con l'URL completo della richiesta API
		slog.Info("Richiesta API Meteo Avanzata", "url", forecastUrl)

		wResp, err := wc.client.Get(forecastUrl)
		if err != nil {
			slog.Error("Errore rete previsioni", "url", forecastUrl, "error", err)
			return WeatherResponse{}, "", fmt.Errorf("forecast network error: %w", err)
		}
		defer wResp.Body.Close()

		var apiMeteo weatherAPIResponse
		if err := json.NewDecoder(wResp.Body).Decode(&apiMeteo); err != nil {
			slog.Error("Errore decodifica previsioni", "error", err)
			return WeatherResponse{}, "", fmt.Errorf("forecast decoding error: %w", err)
		}

		// 3. Fetch Air Quality Index
		aqiUrl := fmt.Sprintf("https://air-quality-api.open-meteo.com/v1/air-quality?latitude=%f&longitude=%f&current=european_aqi", loc.Latitude, loc.Longitude)
		aqiResp, err := wc.client.Get(aqiUrl)
		europeanAQI := 0
		if err == nil {
			var apiAQI airQualityAPIResponse
			if json.NewDecoder(aqiResp.Body).Decode(&apiAQI) == nil {
				europeanAQI = int(apiAQI.Current.EuropeanAQI)
			}
			aqiResp.Body.Close()
		}

		// Parsing Orari Alba/Tramonto
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

		var dailyForecasts []DailyForecast
		for i := 0; i < len(apiMeteo.Daily.Time); i++ {
			dailyForecasts = append(dailyForecasts, DailyForecast{
				Date:              apiMeteo.Daily.Time[i],
				MaxTemp:           apiMeteo.Daily.TempMax[i],
				MinTemp:           apiMeteo.Daily.TempMin[i],
				PrecipProbability: int(apiMeteo.Daily.PrecipProbability[i]),
			})
		}

		finalResponse := WeatherResponse{
			Current:   currentData,
			Daily:     dailyForecasts,
			Timestamp: time.Now().UnixMilli(),
		}

		wc.cache.Put(city, finalResponse)
		return finalResponse, fullName, nil
}
