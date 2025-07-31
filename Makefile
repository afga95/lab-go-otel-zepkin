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

# Verificar se os serviços estão funcionando
check-services:
	@echo "Verificando se os serviços estão rodando..."
	@docker-compose ps
	@echo "\nVerificando conectividade dos serviços..."
	@echo "Testando Serviço A (porta 8081):"
	@timeout 5 bash -c 'until curl -s http://localhost:8081/health > /dev/null; do echo "Aguardando Serviço A..."; sleep 1; done' || echo "Serviço A não está respondendo"
	@echo "Testando Serviço B (porta 8082):"
	@timeout 5 bash -c 'until curl -s http://localhost:8082/health > /dev/null; do echo "Aguardando Serviço B..."; sleep 1; done' || echo "Serviço B não está respondendo"

# Testar o serviço A com tratamento de erro melhorado
test-service-a:
	@echo "========================================="
	@echo "Testando Serviço A..."
	@echo "========================================="
	@echo "\nTeste 1: CEP válido (01310-100)"
	@curl -s -w "\nStatus: %{http_code}\n" -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "01310100"}' || echo "Erro na requisição"
	@echo "\n-----------------------------------------"
	@echo "Teste 2: CEP inválido (formato)"
	@curl -s -w "\nStatus: %{http_code}\n" -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "123"}' || echo "Erro na requisição"
	@echo "\n-----------------------------------------"
	@echo "Teste 3: CEP não encontrado"
	@curl -s -w "\nStatus: %{http_code}\n" -X POST http://localhost:8081 \
		-H "Content-Type: application/json" \
		-d '{"cep": "99999999"}' || echo "Erro na requisição"
	@echo "\n========================================="

# Testar o serviço B diretamente com tratamento de erro melhorado
test-service-b:
	@echo "========================================="
	@echo "Testando Serviço B..."
	@echo "========================================="
	@echo "\nTeste 1: CEP válido (01310-100)"
	@curl -s -w "\nStatus: %{http_code}\n" http://localhost:8082/01310100 || echo "Erro na requisição"
	@echo "\n-----------------------------------------"
	@echo "Teste 2: CEP inválido (formato)"
	@curl -s -w "\nStatus: %{http_code}\n" http://localhost:8082/123 || echo "Erro na requisição"
	@echo "\n-----------------------------------------"
	@echo "Teste 3: CEP não encontrado"
	@curl -s -w "\nStatus: %{http_code}\n" http://localhost:8082/99999999 || echo "Erro na requisição"
	@echo "\n========================================="

# Executar todos os testes
test: check-services test-service-a test-service-b

# Verificar status dos serviços
status:
	@echo "Status dos serviços:"
	docker-compose ps

# Health check dos serviços
health:
	@echo "Health check Serviço A:"
	@curl -s http://localhost:8081/health || echo "Serviço A não está respondendo"
	@echo "\nHealth check Serviço B:"
	@curl -s http://localhost:8082/health || echo "Serviço B não está respondendo"

# Debug - ver logs dos últimas 50 linhas de cada serviço
debug:
	@echo "========================================="
	@echo "Logs do Serviço A (últimas 50 linhas):"
	@echo "========================================="
	@docker-compose logs --tail=50 service-a
	@echo "\n========================================="
	@echo "Logs do Serviço B (últimas 50 linhas):"
	@echo "========================================="
	@docker-compose logs --tail=50 service-b
	@echo "\n========================================="
	@echo "Logs do OTEL Collector (últimas 50 linhas):"
	@echo "========================================="
	@docker-compose logs --tail=50 otel-collector

# Restart dos serviços
restart:
	docker-compose restart

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

# Setup completo (build + up + wait + test)
setup: build up
	@echo "Aguardando serviços iniciarem... (30 segundos)"
	@sleep 30
	@$(MAKE) check-services
	@$(MAKE) health
	@$(MAKE) urls

# Diagnosticar problemas
diagnose:
	@echo "========================================="
	@echo "DIAGNÓSTICO DO SISTEMA"
	@echo "========================================="
	@echo "1. Status dos containers:"
	@docker-compose ps
	@echo "\n2. Portas em uso:"
	@netstat -tlnp | grep -E ':808[12]|:9411|:4317' || echo "Nenhuma porta dos serviços encontrada"
	@echo "\n3. Testando conectividade:"
	@timeout 2 curl -s http://localhost:8081/health && echo "✓ Serviço A OK" || echo "✗ Serviço A não responde"
	@timeout 2 curl -s http://localhost:8082/health && echo "✓ Serviço B OK" || echo "✗ Serviço B não responde"
	@echo "\n4. Logs recentes dos serviços:"
	@$(MAKE) debug