# File: server/server.Dockerfile

# --- Estágio 1: Build ---
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Assumindo que seu go.mod está na raiz e foi corrigido
COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# CORREÇÃO: O caminho para o build agora aponta para o novo ponto de entrada
RUN CGO_ENABLED=0 go build -o /server_app ./cmd/server

# --- Estágio 2: Final ---
FROM scratch
WORKDIR /
COPY --from=builder /server_app .
EXPOSE 8080
CMD ["/server_app"]