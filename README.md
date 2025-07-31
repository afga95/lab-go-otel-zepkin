# Sistema de Temperatura por CEP com OpenTelemetry e Zipkin

Este projeto implementa um sistema distribuído de consulta de temperatura baseado em CEP brasileiro, com tracing distribuído usando OpenTelemetry e Zipkin.

## Arquitetura

O sistema é composto por dois serviços Go:

- **Serviço A (Input Service)**: Responsável por receber e validar CEPs via POST
- **Serviço B (Orchestration Service)**: Responsável por buscar informações do CEP e clima

### Componentes de Observabilidade

- **OpenTelemetry Collector**: Coleta e processa traces
- **Zipkin**: Interface para visualização de traces distribuídos

## Pré-requisitos

- Docker e Docker Compose
- Make (opcional, para comandos facilitados)
- curl e jq (para testes)

## Como executar

### 1. Clonar o repositório

```bash
git clone <repository-url>
cd lab-go-otel-zipkin
```

### 2. Executar com Docker Compose

```bash
# Opção 1: Usando Make (recomendado)
make build 

make up

make test

make down

make clean

```

## Endpoints

### Serviço A (Porta 8081)

- **POST /** - Receber CEP para consulta
- **GET /health** - Health check
- **GET /** - Informações da API

### Serviço B (Porta 8082)

- **GET /{cep}** - Consultar temperatura por CEP
- **GET /health** - Health check
- **GET /** - Informações da API

### Zipkin UI

- **http://localhost:9411** - Interface de visualização de traces

## Testando o Sistema

### Testes Automatizados

```bash
# Testar todos os serviços
make test

# Testar apenas Serviço A
make test-service-a

# Testar apenas Serviço B
make test-service-b
```

### Testes Manuais

#### 1. Teste com CEP válido (Serviço A)

```bash
curl -X POST http://localhost:8081 \
  -H "Content-Type: application/json" \
  -d '{"cep": "01310100"}'
```

**Resposta esperada (200):**
```json
{
  "city": "São Paulo",
  "temp_C": 25.5,
  "temp_F": 77.9,
  "temp_K": 298.5
}
```

#### 2. Teste com CEP inválido (formato)

```bash
curl -X POST http://localhost:8081 \
  -H "Content-Type: application/json" \
  -d '{"cep": "123"}'
```

**Resposta esperada (422):**
```json
{
  "message": "invalid zipcode"
}
```

#### 3. Teste com CEP não encontrado

```bash
curl -X POST http://localhost:8081 \
  -H "Content-Type: application/json" \
  -d '{"cep": "99999999"}'
```

**Resposta esperada (404):**
```json
{
  "message": "can not find zipcode"
}
```

## Visualizando Traces

1. Acesse o Zipkin UI: http://localhost:9411
2. Clique em "Run Query" para ver os traces
3. Clique em um trace específico para ver detalhes
4. Você verá:
   - Spans do Serviço A (cep_handler, call_service_b)
   - Spans do Serviço B (weather_handler, get_cep_info, get_weather_info)
   - Tempo de resposta de cada operação
   - Propagação de contexto entre serviços

## Estrutura do Projeto

```
lab-go-otel-zipkin/
├── docker-compose.yml              # Configuração dos containers
├── otel-collector-config.yml       # Configuração do OpenTelemetry Collector
├── Makefile                        # Comandos facilitados
├── README.md                       # Esta documentação
├── service-a/                      # Serviço A (Input)
│   ├── main.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
└── service-b/                      # Serviço B (Orchestration)
    ├── main.go
    ├── go.mod
    ├── go.sum
    └── Dockerfile
```

## Funcionalidades de Tracing

### Spans Implementados

**Serviço A:**
- `cep_handler`: Handler principal para receber CEP
- `call_service_b`: Chamada HTTP para o Serviço B

**Serviço B:**
- `weather_handler`: Handler principal de orquestração
- `get_cep_info`: Busca informações do CEP na API ViaCEP
- `get_weather_info`: Busca informações climáticas na WeatherAPI

### Atributos de Tracing

Cada span inclui atributos relevantes como:
- CEP consultado
- Status codes HTTP
- Nomes de cidades encontradas
- Temperaturas obtidas
- APIs utilizadas
- Indicadores de sucesso/erro

## Comandos Make Disponíveis

```bash
make build          # Build dos containers
make up             # Subir todos os serviços
make down           # Parar todos os serviços
make logs           # Ver logs de todos os serviços
make test           # Executar todos os testes
make health         # Health check dos serviços
make status         # Status dos containers
make clean          # Limpar containers e volumes
make setup          # Setup completo (build + up + health check)
make urls           # Mostrar URLs úteis
```

## Configurações

### Variáveis de Ambiente

**Serviço A:**
- `PORT`: Porta do servidor (default: 8080)
- `SERVICE_B_URL`: URL do Serviço B
- `OTEL_EXPORTER_OTLP_ENDPOINT`: Endpoint do collector OTLP
- `OTEL_SERVICE_NAME`: Nome do serviço para tracing

**Serviço B:**
- `PORT`: Porta do servidor (default: 8080)
- `WEATHER_API_KEY`: Chave da API WeatherAPI
- `OTEL_EXPORTER_OTLP_ENDPOINT`: Endpoint do collector OTLP
- `OTEL_SERVICE_NAME`: Nome do serviço para tracing

### APIs Externas Utilizadas

1. **ViaCEP**: https://viacep.com.br/ws/{cep}/json/
   - Busca informações de endereço por CEP

2. **WeatherAPI**: http://api.weatherapi.com/v1/current.json
   - Busca informações climáticas atuais
   - Requer chave de API (já configurada)


## Desenvolvimento

Para desenvolvimento local:

```bash
# Executar com rebuild automático
make dev

# Ou manualmente
docker-compose up --build
```

### Modificando o Código

1. Faça as alterações nos arquivos Go
2. Execute `make dev` para rebuild e restart
3. Teste as alterações com `make test`

## Métricas e Monitoramento

- **OpenTelemetry Collector Metrics**: http://localhost:8888/metrics
- **Zipkin UI**: http://localhost:9411
- **Health Checks**: 
  - http://localhost:8081/health
  - http://localhost:8082/health

Este sistema fornece observabilidade completa do fluxo de requisições, permitindo identificar gargalos, erros e tempo de resposta de cada componente da aplicação distribuída.