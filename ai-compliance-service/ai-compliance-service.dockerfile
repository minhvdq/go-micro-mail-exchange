FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o complianceApp ./cmd/api

FROM alpine:3.21
RUN mkdir /app
COPY --from=builder /app/complianceApp /app
CMD ["/app/complianceApp"]
