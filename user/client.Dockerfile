# File: user/client.Dockerfile

# --- Estágio 1: Build ---
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .

# CORREÇÃO: O caminho para o build agora aponta para o novo ponto de entrada
RUN CGO_ENABLED=0 go build -o /client_app ./cmd/client


# --- Estágio 2: Final ---
FROM scratch
WORKDIR /
COPY --from=builder /client_app .
CMD ["/client_app"]