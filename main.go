package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type CEP struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
	Erro        bool   `json:"erro,omitempty"` // Campo para detectar CEP não encontrado
}

type WeatherData struct {
	Location struct {
		Name           string  `json:"name"`
		Region         string  `json:"region"`
		Country        string  `json:"country"`
		Lat            float64 `json:"lat"`
		Lon            float64 `json:"lon"`
		TzID           string  `json:"tz_id"`
		LocaltimeEpoch int     `json:"localtime_epoch"`
		Localtime      string  `json:"localtime"`
	} `json:"location"`
	Current struct {
		LastUpdatedEpoch int     `json:"last_updated_epoch"`
		LastUpdated      string  `json:"last_updated"`
		TempC            float64 `json:"temp_c"`
		TempF            float64 `json:"temp_f"`
		IsDay            int     `json:"is_day"`
		Condition        struct {
			Text string `json:"text"`
			Icon string `json:"icon"`
			Code int    `json:"code"`
		} `json:"condition"`
		WindMph    float64 `json:"wind_mph"`
		WindKph    float64 `json:"wind_kph"`
		WindDegree int     `json:"wind_degree"`
		WindDir    string  `json:"wind_dir"`
		PressureMb float64 `json:"pressure_mb"`
		PressureIn float64 `json:"pressure_in"`
		PrecipMm   float64 `json:"precip_mm"`
		PrecipIn   float64 `json:"precip_in"`
		Humidity   int     `json:"humidity"`
		Cloud      int     `json:"cloud"`
		FeelslikeC float64 `json:"feelslike_c"`
		FeelslikeF float64 `json:"feelslike_f"`
		WindchillC float64 `json:"windchill_c"`
		WindchillF float64 `json:"windchill_f"`
		HeatindexC float64 `json:"heatindex_c"`
		HeatindexF float64 `json:"heatindex_f"`
		DewpointC  float64 `json:"dewpoint_c"`
		DewpointF  float64 `json:"dewpoint_f"`
		VisKm      float64 `json:"vis_km"`
		VisMiles   float64 `json:"vis_miles"`
		Uv         float64 `json:"uv"`
		GustMph    float64 `json:"gust_mph"`
		GustKph    float64 `json:"gust_kph"`
	} `json:"current"`
}

// Estrutura de resposta da API conforme especificação
type TemperatureResponse struct {
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

// Cliente HTTP com timeout
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

func main() {
	// Configuração da porta do servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Configuração das rotas
	r := mux.NewRouter()

	// Rota principal para consulta de CEP e clima
	r.HandleFunc("/{cep}", weatherHandler).Methods("GET")

	// Rota raiz com informações da API
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "CEP Weather API",
			"version": "1.0.0",
			"endpoints": map[string]string{
				"weather": "GET /{cep}",
			},
			"description": "API para consultar temperatura baseada em CEP brasileiro",
		})
	}).Methods("GET")

	// Log de inicialização
	log.Printf("Servidor iniciando na porta %s", port)
	log.Printf("Endpoints disponíveis:")
	log.Printf("  GET /{cep} - Consultar clima por CEP")
	log.Printf("  GET /      - Informações da API")

	// Inicia o servidor
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

// Validação de CEP (deve ter exatamente 8 dígitos)
func isValidCEP(cep string) bool {
	// Remove traços e espaços
	cep = strings.ReplaceAll(cep, "-", "")
	cep = strings.TrimSpace(cep)

	// Verifica se tem exatamente 8 dígitos
	matched, _ := regexp.MatchString(`^\d{8}$`, cep)
	return matched
}

// Busca informações do CEP (baseado no seu código)
func getCEPInfo(cep string) (*CEP, error) {
	// Remove traços para padronizar
	cep = strings.ReplaceAll(cep, "-", "")

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar CEP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erro na API ViaCEP: status %d", resp.StatusCode)
	}

	var cepData CEP
	if err := json.NewDecoder(resp.Body).Decode(&cepData); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta do CEP: %w", err)
	}

	// ViaCEP retorna erro=true quando CEP não é encontrado
	if cepData.Erro {
		return nil, fmt.Errorf("CEP não encontrado")
	}

	// Verifica se a localidade foi encontrada
	if cepData.Localidade == "" {
		return nil, fmt.Errorf("CEP não encontrado")
	}

	return &cepData, nil
}

// Busca informações climáticas (baseado no seu código)
func getWeatherInfo(localidade string) (*WeatherData, error) {
	// Chave da API (pode ser configurada via variável de ambiente)
	apiKey := "ad43e5d744964ababd411426252107"

	// Codifica a localidade para a URL
	cidadeEncoded := url.QueryEscape(localidade)
	urlWeatherAPI := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&lang=pt", apiKey, cidadeEncoded)

	resp, err := httpClient.Get(urlWeatherAPI)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar clima: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("erro na API Weather: status %d", resp.StatusCode)
	}

	var weatherData WeatherData
	if err := json.NewDecoder(resp.Body).Decode(&weatherData); err != nil {
		return nil, fmt.Errorf("erro ao decodificar resposta do clima: %w", err)
	}

	return &weatherData, nil
}

// Conversões de temperatura conforme especificação
func celsiusToFahrenheit(c float64) float64 {
	return c*1.8 + 32
}

func celsiusToKelvin(c float64) float64 {
	return c + 273
}

// Handler principal para consulta de CEP e clima
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	// Configura headers de resposta
	w.Header().Set("Content-Type", "application/json")

	// Extrai o CEP da URL
	vars := mux.Vars(r)
	cep := vars["cep"]

	// Validação 1: Formato do CEP (422 - invalid zipcode)
	if !isValidCEP(cep) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	// Busca informações do CEP
	cepInfo, err := getCEPInfo(cep)
	if err != nil {
		// Validação 2: CEP não encontrado (404 - can not find zipcode)
		log.Printf("Erro ao buscar CEP %s: %v", cep, err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "can not find zipcode"})
		return
	}

	// Busca informações climáticas
	weatherInfo, err := getWeatherInfo(cepInfo.Localidade)
	if err != nil {
		log.Printf("Erro ao buscar clima para %s: %v", cepInfo.Localidade, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "weather service unavailable"})
		return
	}

	// Prepara resposta com todas as temperaturas conforme especificação
	tempC := weatherInfo.Current.TempC
	response := TemperatureResponse{
		TempC: tempC,
		TempF: celsiusToFahrenheit(tempC),
		TempK: celsiusToKelvin(tempC),
	}

	// Sucesso: 200 com as temperaturas
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
