-----

# PBL1\_Redes - Jogo de Cartas Online

Este é um projeto de um jogo de cartas multiplayer (Pedra, Papel e Tesoura) implementado em Go. Ele utiliza uma arquitetura cliente-servidor com comunicação via sockets TCP e persistência de dados do usuário em arquivos JSON locais.

## Como o Jogo Funciona

O fluxo do jogo é baseado na interação entre o cliente (jogador) e o servidor através de comandos de texto.

1.  **Conexão e Perfil:**

      * O jogador inicia o cliente e se conecta ao endereço IP do servidor.
      * Se for o primeiro acesso, o programa solicita um nome e cria um arquivo local `user_data.json` para salvar o progresso. Por padrão, um novo jogador **começa com 0 cartas**.
      * Se um perfil já existe, ele é carregado no início.

2.  **Obtendo Cartas:**

      * Como o inventário inicial é zero, o primeiro passo do jogador é usar o comando `pack`.
      * Este comando envia uma solicitação ao servidor, que por sua vez envia 3 cartas aleatórias de seu estoque global para o jogador. O inventário do jogador é atualizado e salvo localmente.

3.  **Entrando em uma Partida:**

      * Com cartas no inventário, o jogador pode digitar o comando `play`.
      * O servidor recebe a solicitação e adiciona o jogador a uma fila de matchmaking.
      * Quando dois jogadores estão na fila, o servidor os une em uma partida e notifica ambos que o jogo começou.

4.  **A Partida (Rodada Única):**

      * O jogo consiste em uma **única rodada** decisiva.
      * Ambos os jogadores devem usar o comando `move <carta>` (ex: `move Rock`, `move Paper`, etc.) para escolher uma carta do seu inventário.
      * O servidor aguarda a jogada de ambos, com um tempo limite para evitar partidas travadas.
      * Assim que as duas jogadas são recebidas, o servidor determina o resultado:
          * **Vitória/Derrota:** Baseado nas regras clássicas (Pedra \> Tesoura \> Papel \> Pedra).
          * **Empate:** Se ambos jogarem a mesma carta.
      * Ocorre uma **troca de cartas**: o vencedor recebe a carta que o perdedor jogou, e o perdedor recebe a carta que o vencedor jogou. Em caso de empate, nada acontece.
      * A partida é finalizada, uma mensagem de `game_over` é enviada com o resultado e o novo inventário de cada jogador, que é salvo localmente. Após isso, os jogadores estão livres para usar `play` novamente.

## JOGO DE CARTAS MULTIPLAYER - GUIA DE EXECUÇÃO COM DOCKER

Este guia fornece as instruções passo a passo para construir e executar o ambiente completo do jogo utilizando a estrutura de Docker e Docker Compose já presente no projeto.

### PRÉ-REQUISITOS

Antes de começar, certifique-se de que você tem os seguintes softwares instalados e em execução na sua máquina:

  * Docker
  * Docker Compose (geralmente incluído com a instalação do Docker Desktop)

### EXECUÇÃO PASSO A PASSO

Todos os comandos devem ser executados a partir do **diretório raiz do projeto** (a pasta que contém o arquivo `docker-compose.yml`).

#### 1\. CONSTRUIR AS IMAGENS (BUILD)

O primeiro passo é construir as imagens para cada um dos serviços (servidor, cliente e testes) a partir de seus respectivos `Dockerfiles`.

```bash
docker-compose build
```

Este comando irá ler o `docker-compose.yml`, encontrar as instruções de `build` para cada serviço e criar as imagens Docker locais.

#### 2\. INICIAR O SERVIDOR (UP)

Com as imagens prontas, você pode iniciar o container do servidor. Usamos a flag `-d` para que ele rode em segundo plano (*detached mode*).

```bash
docker-compose up -d server
```

Neste ponto, o servidor do jogo estará no ar e pronto para aceitar conexões na porta `8080`.

#### 3\. ACESSAR O JOGO (EXECUTAR O CLIENTE)

Para jogar, inicie o container do cliente de forma interativa. Ele se conectará automaticamente ao container do servidor.

```bash
docker-compose run --rm client [IP_OPCIONAL_DO_SERVIDOR]
```

**Análise do Comando:**

  * `docker-compose run`: Inicia uma instância única do serviço `client`.
  * `--rm`: Flag que remove o container automaticamente assim que a sessão do jogo terminar, mantendo o sistema limpo.
  * `client`: O nome do serviço que queremos executar.

Após executar este comando, seu terminal estará conectado ao cliente do jogo, e você poderá usar os comandos como `stock`, `pack`, `play`, etc.

### COMANDOS ADICIONAIS DE GERENCIAMENTO

#### MONITORAMENTO E LOGS

Para visualizar os logs do servidor em tempo real (ver conexões, pareamentos, erros, etc.), utilize o seguinte comando em um novo terminal:

```bash
docker-compose logs -f server
```

  * A flag `-f` (follow) mantém o terminal exibindo novas mensagens de log assim que são geradas pelo servidor.

#### EXECUTAR OS TESTES AUTOMATIZADOS

O ambiente Docker também está configurado para rodar a suíte de testes contra o servidor. Com o servidor em execução (após o `docker-compose up -d server`), execute:

```bash
docker-compose run --rm tests
```

Este comando iniciará o container de testes, que executará o `go test -v ./...` contra o container do servidor e exibirá os resultados.

#### ENCERRAMENTO DO JOGO

Quando terminar, você pode parar e remover todos os containers, redes e outros recursos criados pelo Docker Compose com um único comando:

```bash
docker-compose down
```

### COMO A COMUNICAÇÃO DENTRO DO DOCKER FUNCIONA

O `docker-compose.yml` define uma rede virtual privada chamada `cardgame-net`. Todos os serviços (`server`, `client`, `tests`) são conectados a esta rede, permitindo que eles se comuniquem uns com os outros usando seus nomes de serviço como se fossem nomes de domínio. É por isso que o cliente consegue se conectar ao servidor usando o endereço `server` em vez de um endereço IP. A diretiva `depends_on` garante que o servidor sempre inicie antes dos componentes que precisam dele.

## Configuração Inicial

É possível alterar a quantidade inicial de cartas com que os usuários e o sistema começam.

### Cartas Iniciais do Usuário

Por padrão, um novo usuário começa com **0 cartas** e precisa usar o comando `pack` para adquiri-las. Para alterar a quantidade de cartas que um **novo usuário** recebe ao criar um perfil, você deve editar o arquivo `user/game_user.go`.

1.  Abra o arquivo `user/game_user.go`.

2.  Localize a função `loadData`.

3.  Altere os valores no mapa `Stock`:

    ```go
    // Trecho da função loadData() em user/game_user.go

    currentUserData = UserData{
        Name: strings.TrimSpace(name),
        Stock: map[string]int{
            "Rock":     0, // Altere este valor
            "Paper":    0, // Altere este valor
            "Scissors": 0, // Altere este valor
        },
    }
    ```

    **Exemplo:** Para que cada novo jogador comece com 5 cartas de cada tipo, o código ficaria assim:

    ```go
    Stock: map[string]int{
        "Rock":     5,
        "Paper":    5,
        "Scissors": 5,
    },
    ```

### Cartas Iniciais do Servidor (Estoque Global)

O servidor possui um estoque global de cartas que são distribuídas aos jogadores quando eles usam o comando `pack`. Para alterar este estoque inicial do **servidor**, você deve editar o arquivo `server/game_server.go`.

1.  Abra o arquivo `server/game_server.go`.

2.  Localize a declaração da variável global `globalStock`.

3.  Altere a quantidade de cada carta no mapa `Cards`:

    ```go
    // Trecho das variáveis globais em server/game_server.go

    var (
        // ...
        globalStock = &GlobalCardStock{
            Cards: map[game.Card]int{
                game.Rock: 2, // Altere este valor
                game.Paper: 2, // Altere este valor
                game.Scissors: 2, // Altere este valor
            },
        }
    )
    ```

    No código acima, o servidor pode distribuir no máximo 6 cartas (2 de cada) antes de seu estoque acabar.

    **Exemplo:** Para que o servidor comece com um estoque de 100 cartas de cada tipo:

    ```go
    Cards: map[game.Card]int{
        game.Rock: 100,
        game.Paper: 100,
        game.Scissors: 100,
    },
    ```
