FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o tenantApp ./cmd/api

FROM alpine:latest
RUN mkdir /app
COPY --from=builder /app/tenantApp /app
CMD ["/app/tenantApp"]
