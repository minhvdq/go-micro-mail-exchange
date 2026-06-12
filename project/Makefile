FRONTEND_BINARY=frontendApp
BROKER_BINARY=brokerApp
MAIL_BINARY=mailServiceApp
TENANT_BINARY=tenantApp
COMPLIANCE_BINARY=complianceApp

## up: starts all containers in the background without forcing build
up:
	@echo "Starting Docker images..."
	docker compose up -d
	@echo "Docker images started!"

## up_build: stops docker-compose (if running), builds all projects and starts docker compose
up_build: build_broker build_mail build_tenant build_compliance build_frontend
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

## build_frontend: builds the frontend binary as a linux executable
build_frontend:
	@echo "Building frontend binary..."
	cd ../front-end && env GOOS=linux CGO_ENABLED=0 go build -o ${FRONTEND_BINARY} ./cmd/web
	@echo "Done!"

## start: builds and runs the front end locally (dev)
start: build_front
	@echo "Starting front end"
	cd ../front-end && ./${FRONTEND_BINARY} &

## stop: stop the front end
stop:
	@echo "Stopping front end..."
	@-pkill -SIGTERM -f "./${FRONTEND_BINARY}"
	@echo "Stopped front end!"

restart:
	make stop
	make start
