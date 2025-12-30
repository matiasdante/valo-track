.PHONY: build run update analyze help clean install

# Variables
BINARY_NAME=tracker
GO_FILES=$(shell find . -name '*.go' -type f)

# Colores para output
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

help: ## Mostrar ayuda
	@echo "$(GREEN)Valorant Competitive Tracker$(NC)"
	@echo "=============================="
	@echo ""
	@echo "Comandos disponibles:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""

install: ## Instalar dependencias
	@echo "$(GREEN)Instalando dependencias...$(NC)"
	go mod download
	go mod verify
	@echo "$(GREEN)✅ Dependencias instaladas$(NC)"

build: ## Compilar el proyecto
	@echo "$(GREEN)Compilando...$(NC)"
	go build -o $(BINARY_NAME) cmd/tracker/main.go
	@echo "$(GREEN)✅ Compilado: ./$(BINARY_NAME)$(NC)"

run: build ## Compilar y ejecutar con análisis
	@echo "$(GREEN)Ejecutando análisis...$(NC)"
	./$(BINARY_NAME) -analyze

update: build ## Actualizar datos desde API
	@echo "$(GREEN)Actualizando datos desde Valorant API...$(NC)"
	./$(BINARY_NAME) -update

full: build ## Actualización completa + análisis mensual
	@echo "$(GREEN)Actualización y análisis completo...$(NC)"
	./$(BINARY_NAME) -update -analyze -timeframe=monthly

weekly: build ## Análisis semanal
	@echo "$(GREEN)Análisis semanal...$(NC)"
	./$(BINARY_NAME) -analyze -timeframe=weekly

recent: build ## Últimos 10 matches
	@echo "$(GREEN)Análisis de últimos 10 matches...$(NC)"
	./$(BINARY_NAME) -analyze -timeframe=recent -recent=10

season: build ## Análisis por season
	@echo "$(GREEN)Análisis por season...$(NC)"
	./$(BINARY_NAME) -analyze -timeframe=season

clean: ## Limpiar archivos generados
	@echo "$(GREEN)Limpiando...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f *.xlsx
	@echo "$(GREEN)✅ Limpieza completa$(NC)"

clean-data: ## Limpiar datos históricos (¡cuidado!)
	@echo "$(YELLOW)⚠️  Esto eliminará todos los datos históricos$(NC)"
	@read -p "¿Estás seguro? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		rm -f data/history.json; \
		echo "$(GREEN)✅ Datos eliminados$(NC)"; \
	else \
		echo "$(YELLOW)Cancelado$(NC)"; \
	fi

test: ## Ejecutar tests
	@echo "$(GREEN)Ejecutando tests...$(NC)"
	go test -v ./...

lint: ## Ejecutar linter
	@echo "$(GREEN)Ejecutando golangci-lint...$(NC)"
	golangci-lint run ./...

check: lint test ## Ejecutar checks (lint + test)
	@echo "$(GREEN)✅ Todos los checks pasaron$(NC)"

# Comandos de desarrollo
dev-update: ## Actualizar con pocos datos (dev)
	./$(BINARY_NAME) -update -region=latam

dev-analyze: ## Análisis rápido (últimos 5)
	./$(BINARY_NAME) -analyze -timeframe=recent -recent=5

# Info
info: ## Mostrar información del proyecto
	@echo "$(GREEN)Valorant Competitive Tracker$(NC)"
	@echo "=============================="
	@echo "Binary: $(BINARY_NAME)"
	@echo "Go Version: $(shell go version)"
	@echo "Files: $(shell echo $(GO_FILES) | wc -w) archivos Go"
	@echo ""
	@echo "Estructura:"
	@tree -L 2 -I 'vendor|bin' || ls -R

backup: ## Crear backup del histórico
	@echo "$(GREEN)Creando backup...$(NC)"
	@mkdir -p backups
	@cp data/history.json backups/history_$(shell date +%Y%m%d_%H%M%S).json 2>/dev/null || true
	@echo "$(GREEN)✅ Backup creado en backups/$(NC)"

.DEFAULT_GOAL := help
