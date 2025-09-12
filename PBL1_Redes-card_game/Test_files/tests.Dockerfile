# File: tests.Dockerfile

# Usamos a imagem completa do Go, pois precisamos das ferramentas de teste
FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

# O comando padrão executará todos os testes no projeto
# O servidor precisa estar acessível pelo nome 'server' na rede do compose
CMD ["go", "test", "-v", "./..."]