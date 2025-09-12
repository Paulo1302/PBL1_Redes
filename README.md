

-----

# JOGO DE CARTAS MULTIPLAYER - GUIA DE EXECUÇÃO COM DOCKER

Este guia fornece as instruções passo a passo para construir e executar o ambiente completo do jogo utilizando a estrutura de Docker e Docker Compose já presente no projeto.

## PRÉ-REQUISITOS

Antes de começar, certifique-se de que você tem os seguintes softwares instalados e em execução na sua máquina:

  * Docker
  * Docker Compose (geralmente incluído com a instalação do Docker Desktop)

## EXECUÇÃO PASSO A PASSO

Todos os comandos devem ser executados a partir do **diretório raiz do projeto** (a pasta que contém o arquivo `docker-compose.yml`).

### 1\. CONSTRUIR AS IMAGENS (BUILD)

O primeiro passo é construir as imagens para cada um dos serviços (servidor, cliente e testes) a partir de seus respectivos `Dockerfiles`.

```bash
docker-compose build
```

Este comando irá ler o `docker-compose.yml`, encontrar as instruções de `build` para cada serviço e criar as imagens Docker locais.

### 2\. INICIAR O SERVIDOR (UP)

Com as imagens prontas, você pode iniciar o container do servidor. Usamos a flag `-d` para que ele rode em segundo plano (*detached mode*).

```bash
docker-compose up -d server
```

Neste ponto, o servidor do jogo estará no ar e pronto para aceitar conexões na porta `8080`.

### 3\. ACESSAR O JOGO (EXECUTAR O CLIENTE)

Para jogar, inicie o container do cliente de forma interativa. Ele se conectará automaticamente ao container do servidor.

```bash
docker-compose run --rm client server
```

**Análise do Comando:**

  * `docker-compose run`: Inicia uma instância única do serviço `client`.
  * `--rm`: Flag que remove o container automaticamente assim que a sessão do jogo terminar, mantendo o sistema limpo.
  * `client`: O nome do serviço que queremos executar.
  * `server`: Este argumento é passado para o programa cliente, informando que o endereço do servidor é `server` dentro da rede Docker.

Após executar este comando, seu terminal estará conectado ao cliente do jogo, e você poderá usar os comandos como `stock`, `pack`, `play`, etc.

## COMANDOS ADICIONAIS DE GERENCIAMENTO

### MONITORAMENTO E LOGS

Para visualizar os logs do servidor em tempo real (ver conexões, pareamentos, erros, etc.), utilize o seguinte comando em um novo terminal:

```bash
docker-compose logs -f server
```

  * A flag `-f` (follow) mantém o terminal exibindo novas mensagens de log assim que são geradas pelo servidor.

### EXECUTAR OS TESTES AUTOMATIZADOS

O ambiente Docker também está configurado para rodar a suíte de testes contra o servidor. Com o servidor em execução (após o `docker-compose up -d server`), execute:

```bash
docker-compose run --rm tests
```

[cite\_start]Este comando iniciará o container de testes, que executará o `go test -v ./...` contra o container do servidor e exibirá os resultados[cite: 1].

### ENCERRAMENTO DO JOGO

Quando terminar, você pode parar e remover todos os containers, redes e outros recursos criados pelo Docker Compose com um único comando:

```bash
docker-compose down
```

## COMO A COMUNICAÇÃO FUNCIONA

[cite\_start]O `docker-compose.yml` define uma rede virtual privada chamada `cardgame-net`[cite: 1]. [cite\_start]Todos os serviços (`server`, `client`, `tests`) são conectados a esta rede, permitindo que eles se comuniquem uns com os outros usando seus nomes de serviço como se fossem nomes de domínio[cite: 1]. É por isso que o cliente consegue se conectar ao servidor usando o endereço `server` em vez de um endereço IP. [cite\_start]A diretiva `depends_on` garante que o servidor sempre inicie antes dos componentes que precisam dele[cite: 1].
