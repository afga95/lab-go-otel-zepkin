package main

import (
	"context"
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
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	Erro        bool   `json:"erro,omitempty"`
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

type TemperatureResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

var (
	httpClient    *http.Client
	tracer        trace.Tracer
	weatherAPIKey string
)

func main() {
	// Inicializar OpenTelemetry
	if err := initTracer(); err != nil {
		log.Fatalf("Erro ao inicializar tracer: %v", err)
	}

	// Configuração da porta do servidor
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Weather API Key
	weatherAPIKey = os.Getenv("WEATHER_API_KEY")
	if weatherAPIKey == "" {
		weatherAPIKey = "ad43e5d744964ababd411426252107"
	}

	// Cliente HTTP com instrumentação OpenTelemetry
	httpClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// Tracer
	tracer = otel.Tracer("service-b")

	// Configuração das rotas
	r := mux.NewRouter()
	r.Use(otelmux.Middleware("service-b"))

	// Rota principal para consulta de CEP e clima
	r.HandleFunc("/{cep}", weatherHandler).Methods("GET")

	// Rota de health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// Rota raiz com informações da API
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":     "CEP Weather API",
			"version":     "1.0.0",
			"description": "Serviço B - Responsável pela orquestração de CEP e clima",
			"endpoints": map[string]string{
				"weather": "GET /{cep}",
				"health":  "GET /health",
			},
		})
	}).Methods("GET")

	// Log de inicialização
	log.Printf("Serviço B iniciando na porta %s", port)
	log.Printf("Weather API Key configurada: %v", weatherAPIKey != "")
	log.Printf("Endpoints disponíveis:")
	log.Printf("  GET /{cep}  - Consultar clima por CEP")
	log.Printf("  GET /health - Health check")

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

// Inicializa o OpenTelemetry tracer
func initTracer() error {
	// Configuração do resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("service-b"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return fmt.Errorf("erro ao criar resource: %w", err)
	}

	// Endpoint do collector
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "localhost:4317"
	}

	// Configuração do exporter OTLP usando grpc.NewClient
	conn, err := grpc.NewClient(
		otlpEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("erro ao conectar com OTLP endpoint: %w", err)
	}

	exporter, err := otlptracegrpc.New(context.Background(), otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return fmt.Errorf("erro ao criar exporter: %w", err)
	}

	// Configuração do trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return nil
}

// Validação de CEP
func isValidCEP(cep string) bool {
	cep = strings.ReplaceAll(cep, "-", "")
	cep = strings.TrimSpace(cep)
	matched, _ := regexp.MatchString(`^\d{8}$`, cep)
	return matched
}

// Busca informações do CEP com tracing
func getCEPInfo(ctx context.Context, cep string) (*CEP, error) {
	ctx, span := tracer.Start(ctx, "get_cep_info")
	defer span.End()

	span.SetAttributes(
		attribute.String("cep", cep),
		attribute.String("api", "viacep"),
	)

	// Remove traços para padronizar
	cep = strings.ReplaceAll(cep, "-", "")
	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao consultar CEP: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("erro na API ViaCEP: status %d", resp.StatusCode)
		span.RecordError(err)
		return nil, err
	}

	var cepData CEP
	if err := json.NewDecoder(resp.Body).Decode(&cepData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao decodificar resposta do CEP: %w", err)
	}

	// ViaCEP retorna erro=true quando CEP não é encontrado
	if cepData.Erro {
		err := fmt.Errorf("CEP não encontrado")
		span.SetAttributes(attribute.Bool("cep.found", false))
		return nil, err
	}

	// Verifica se a localidade foi encontrada
	if cepData.Localidade == "" {
		err := fmt.Errorf("CEP não encontrado")
		span.SetAttributes(attribute.Bool("cep.found", false))
		return nil, err
	}

	span.SetAttributes(
		attribute.Bool("cep.found", true),
		attribute.String("localidade", cepData.Localidade),
		attribute.String("uf", cepData.Uf),
	)

	return &cepData, nil
}

// Busca informações climáticas com tracing
func getWeatherInfo(ctx context.Context, localidade string) (*WeatherData, error) {
	ctx, span := tracer.Start(ctx, "get_weather_info")
	defer span.End()

	span.SetAttributes(
		attribute.String("localidade", localidade),
		attribute.String("api", "weatherapi"),
	)

	// Codifica a localidade para a URL
	cidadeEncoded := url.QueryEscape(localidade)
	urlWeatherAPI := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s&lang=pt", weatherAPIKey, cidadeEncoded)

	req, err := http.NewRequestWithContext(ctx, "GET", urlWeatherAPI, nil)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao consultar clima: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("erro na API Weather: status %d", resp.StatusCode)
		span.RecordError(err)
		return nil, err
	}

	var weatherData WeatherData
	if err := json.NewDecoder(resp.Body).Decode(&weatherData); err != nil {
		span.RecordError(err)
		return nil, fmt.Errorf("erro ao decodificar resposta do clima: %w", err)
	}

	span.SetAttributes(
		attribute.String("weather.location", weatherData.Location.Name),
		attribute.Float64("weather.temp_c", weatherData.Current.TempC),
		attribute.String("weather.condition", weatherData.Current.Condition.Text),
	)

	return &weatherData, nil
}

// Conversões de temperatura
func celsiusToFahrenheit(c float64) float64 {
	return c*1.8 + 32
}

func celsiusToKelvin(c float64) float64 {
	return c + 273
}

// Handler principal para consulta de CEP e clima
func weatherHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Inicia span para o handler
	ctx, span := tracer.Start(ctx, "weather_handler")
	defer span.End()

	// Configura headers de resposta
	w.Header().Set("Content-Type", "application/json")

	// Extrai o CEP da URL
	vars := mux.Vars(r)
	cep := vars["cep"]

	span.SetAttributes(attribute.String("cep", cep))

	// Validação 1: Formato do CEP (422 - invalid zipcode)
	if !isValidCEP(cep) {
		span.SetAttributes(attribute.String("validation", "invalid_zipcode"))
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	// Busca informações do CEP
	cepInfo, err := getCEPInfo(ctx, cep)
	if err != nil {
		// Validação 2: CEP não encontrado (404 - can not find zipcode)
		log.Printf("Erro ao buscar CEP %s: %v", cep, err)
		span.RecordError(err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "can not find zipcode"})
		return
	}

	// Busca informações climáticas
	weatherInfo, err := getWeatherInfo(ctx, cepInfo.Localidade)
	if err != nil {
		log.Printf("Erro ao buscar clima para %s: %v", cepInfo.Localidade, err)
		span.RecordError(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "weather service unavailable"})
		return
	}

	// Prepara resposta com todas as temperaturas conforme especificação
	tempC := weatherInfo.Current.TempC
	response := TemperatureResponse{
		City:  weatherInfo.Location.Name,
		TempC: tempC,
		TempF: celsiusToFahrenheit(tempC),
		TempK: celsiusToKelvin(tempC),
	}

	// Adiciona informações ao span
	span.SetAttributes(
		attribute.String("response.city", response.City),
		attribute.Float64("response.temp_c", response.TempC),
		attribute.Float64("response.temp_f", response.TempF),
		attribute.Float64("response.temp_k", response.TempK),
	)

	// Sucesso: 200 com as temperaturas
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
