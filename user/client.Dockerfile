# --- ESTÁGIO 1: Construtor (Builder) ---
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copia todos os arquivos do projeto para o diretório de trabalho.
# Fazemos isso primeiro, pois o go.sum pode não existir localmente.
COPY . .

# O comando 'go mod tidy' garante que o go.mod e o go.sum estejam
# consistentes, baixando as dependências necessárias e criando o go.sum.
RUN go mod tidy

# Agora, compila o código do cliente.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/client-exec ./cmd/client


# --- ESTÁGIO 2: Final ---
FROM scratch

WORKDIR /

# Copia o executável compilado.
COPY --from=builder /app/client-exec /client-exec

# Define o ponto de entrada.
ENTRYPOINT ["/client-exec"]
