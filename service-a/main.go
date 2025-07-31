package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

// Estruturas de dados
type CEPRequest struct {
	CEP string `json:"cep"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type TemperatureResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

var (
	httpClient  *http.Client
	serviceBURL string
	tracer      trace.Tracer
)

func main() {

	// Inicializar OpenTelemetry
	if err := initTracer(); err != nil {
		log.Fatalf("Erro ao inicializar tracer: %v", err)
	}

	// Configurações
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	serviceBURL = os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		serviceBURL = "http://localhost:8082"
	}

	// Cliente HTTP com instrumentação OpenTelemetry
	httpClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	// Tracer
	tracer = otel.Tracer("service-a")

	// Configuração das rotas
	r := mux.NewRouter()
	r.Use(otelmux.Middleware("service-a"))

	// Rota principal para receber CEP
	r.HandleFunc("/", cepHandler).Methods("POST")

	// Rota de health check
	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}).Methods("GET")

	// Rota raiz com informações da API
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":     "CEP Input Service",
			"version":     "1.0.0",
			"description": "Serviço A - Responsável por receber e validar CEPs",
			"endpoints": map[string]string{
				"input":  "POST / - Receber CEP",
				"health": "GET /health - Health check",
			},
		})
	}).Methods("GET")

	// Log de inicialização
	log.Printf("Serviço A iniciando na porta %s", port)
	log.Printf("Service B URL: %s", serviceBURL)
	log.Printf("Endpoints disponíveis:")
	log.Printf("  POST /      - Receber CEP")
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
			semconv.ServiceNameKey.String("service-a"),
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

	// Configuração do exporter OTLP
	conn, err := grpc.DialContext(
		context.Background(),
		otlpEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("erro ao conectar com OTLP endpoint: %w", err)
	}
	fmt.Println("cheguei ate aqui")

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
func isValidCEPFormat(cep string) bool {
	// Remove traços e espaços
	cep = strings.ReplaceAll(cep, "-", "")
	cep = strings.TrimSpace(cep)

	// Verifica se tem exatamente 8 dígitos
	matched, _ := regexp.MatchString(`^\d{8}$`, cep)
	return matched
}

// Handler principal para receber CEP
func cepHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Inicia span para o handler
	ctx, span := tracer.Start(ctx, "cep_handler")
	defer span.End()

	// Configura headers de resposta
	w.Header().Set("Content-Type", "application/json")

	// Decodifica o JSON do request
	var cepReq CEPRequest
	if err := json.NewDecoder(r.Body).Decode(&cepReq); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error", "invalid_json"))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid request body"})
		return
	}

	// Adiciona CEP ao span
	span.SetAttributes(attribute.String("cep", cepReq.CEP))

	// Validação: CEP deve ser string e ter formato válido
	if cepReq.CEP == "" || !isValidCEPFormat(cepReq.CEP) {
		span.SetAttributes(attribute.String("validation", "invalid_zipcode"))
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		return
	}

	// Chama o Serviço B
	response, err := callServiceB(ctx, cepReq.CEP)
	if err != nil {
		span.RecordError(err)

		// Trata diferentes tipos de erro do Serviço B
		if strings.Contains(err.Error(), "invalid zipcode") {
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "invalid zipcode"})
		} else if strings.Contains(err.Error(), "can not find zipcode") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "can not find zipcode"})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{Message: "internal server error"})
		}
		return
	}

	// Sucesso
	span.SetAttributes(
		attribute.String("city", response.City),
		attribute.Float64("temp_c", response.TempC),
	)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Chama o Serviço B
func callServiceB(ctx context.Context, cep string) (*TemperatureResponse, error) {
	// Inicia span para chamada ao Serviço B
	ctx, span := tracer.Start(ctx, "call_service_b")
	defer span.End()

	span.SetAttributes(
		attribute.String("service", "service-b"),
		attribute.String("cep", cep),
	)

	// Monta a URL
	url := fmt.Sprintf("%s/%s", serviceBURL, cep)

	// Cria request com contexto
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar request: %w", err)
	}

	// Faz a chamada HTTP
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar serviço B: %w", err)
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	// Trata diferentes códigos de status
	switch resp.StatusCode {
	case http.StatusOK:
		var tempResp TemperatureResponse
		if err := json.NewDecoder(resp.Body).Decode(&tempResp); err != nil {
			return nil, fmt.Errorf("erro ao decodificar resposta: %w", err)
		}
		return &tempResp, nil

	case http.StatusUnprocessableEntity:
		return nil, fmt.Errorf("invalid zipcode")

	case http.StatusNotFound:
		return nil, fmt.Errorf("can not find zipcode")

	default:
		return nil, fmt.Errorf("erro no serviço B: status %d", resp.StatusCode)
	}
}
