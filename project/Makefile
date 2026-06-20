BROKER_BINARY=brokerApp
MAIL_BINARY=mailServiceApp
TENANT_BINARY=tenantApp
COMPLIANCE_BINARY=complianceApp

## migrate: applies all SQL migrations in order against the running postgres container
migrate:
	@echo "Applying migrations..."
	@for f in $(shell ls ../db-migrations/*.sql | sort); do \
		echo "  -> $$f"; \
		docker exec -i $$(docker ps -qf "name=postgres") psql -U postgres -d users < $$f; \
	done
	@echo "Migrations done!"

## up: starts all containers in the background without forcing build
up:
	@echo "Starting Docker images..."
	docker compose up -d
	@echo "Docker images started!"

## up_build: stops docker-compose (if running), builds all projects and starts docker compose
up_build: build_broker build_mail build_tenant build_compliance
	@echo "Stopping docker images (if running...)"
	docker compose down
	@echo "Building (when required) and starting docker images..."
	docker compose up --build -d
	@echo "Docker images built and started!"

## down: stop docker compose
down:
	@echo "Stopping docker compose..."
	docker compose down
	@echo "Done!"

## build_broker: builds the broker binary as a linux executable
build_broker:
	@echo "Building broker binary..."
	cd ../broker-service && env GOOS=linux CGO_ENABLED=0 go build -o ${BROKER_BINARY} ./cmd/api
	@echo "Done!"

## build_mail: builds the mail binary as a linux executable
build_mail:
	@echo "Building mail binary..."
	cd ../mail-service && env GOOS=linux CGO_ENABLED=0 go build -o ${MAIL_BINARY} ./cmd/api
	@echo "Done!"

## build_tenant: builds the tenant-service binary as a linux executable
build_tenant:
	@echo "Building tenant service binary..."
	cd ../tenant-service && env GOOS=linux CGO_ENABLED=0 go build -o ${TENANT_BINARY} ./cmd/api
	@echo "Done!"

## build_compliance: builds the ai-compliance-service binary as a linux executable
build_compliance:
	@echo "Building ai-compliance-service binary..."
	cd ../ai-compliance-service && env GOOS=linux CGO_ENABLED=0 go build -o ${COMPLIANCE_BINARY} ./cmd/api
	@echo "Done!"

## build_frontend: builds the React frontend for production (output: front-end/dist/)
build_frontend:
	@echo "Building frontend..."
	cd ../front-end && npm ci && npm run build
	@echo "Done!"

## start: starts the Vite dev server on port 3000
start:
	@echo "Starting frontend dev server..."
	cd ../front-end && npm run dev &

## stop: stop the Vite dev server
stop:
	@echo "Stopping frontend dev server..."
	@-pkill -f "vite"
	@echo "Done!"
