.PHONY: build up down logs test clean dev

# Build dos serviços
build:
	docker-compose build

# Subir toda a stack
up:
	docker-compose up -d

# Parar todos os serviços
down:
	docker-compose down

# Ver logs de todos os serviços
logs:
	docker-compose logs -f

# Ver logs de um serviço específico
logs-service-a:
	docker-compose logs -f service-a

logs-service-b:
	docker-compose logs -f service-b

logs-zipkin:
	docker-compose logs -f zipkin

logs-otel:
	docker-compose logs -f otel-collector

# Executar em modo desenvolvimento (com rebuild)
dev:
	docker-compose up --build

# Testar o serviço A
test-service-a:
	@echo "Testando Serviço A..."
	@echo "Teste 1: CEP válido (01310-100)"
	curl -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "01310100"}' | jq .
	@echo "\nTeste 2: CEP inválido (formato)"
	curl -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "123"}' | jq .
	@echo "\nTeste 3: CEP não encontrado"
	curl -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "99999999"}' | jq .

# Testar o serviço B diretamente
test-service-b:
	@echo "Testando Serviço B..."
	@echo "Teste 1: CEP válido (01310-100)"
	curl http://localhost:8082/01310100 | jq .
	@echo "\nTeste 2: CEP inválido (formato)"
	curl http://localhost:8082/123 | jq .
	@echo "\nTeste 3: CEP não encontrado"
	curl http://localhost:8082/99999999 | jq .

# Executar todos os testes
test: test-service-a test-service-b

# Verificar status dos serviços
status:
	@echo "Status dos serviços:"
	docker-compose ps

# Health check dos serviços
health:
	@echo "Health check Serviço A:"
	curl http://localhost:8081/health | jq .
	@echo "\nHealth check Serviço B:"
	curl http://localhost:8082/health | jq .

# Abrir Zipkin no navegador (macOS)
zipkin:
	open http://localhost:9411

# Mostrar URLs úteis
urls:
	@echo "URLs dos serviços:"
	@echo "Serviço A: http://localhost:8081"
	@echo "Serviço B: http://localhost:8082"
	@echo "Zipkin UI: http://localhost:9411"
	@echo "OTEL Collector metrics: http://localhost:8888/metrics"

# Limpar containers e volumes
clean:
	docker-compose down -v
	docker system prune -f

# Setup completo (build + up + test)
setup: build up
	@echo "Aguardando serviços iniciarem..."
	sleep 10
	$(MAKE) health
	$(MAKE) urls