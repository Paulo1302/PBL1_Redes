# PBL1_Redes
Primeiro problema da disciplina TEC 502 concorrência e conectividade "Jogo de Cartas Multiplayer" desenvolvido em Go
Guia de Execução do Jogo com Docker

1. Iniciar o Ambiente com Docker

O processo de inicialização é dividido em duas etapas: construir as imagens (só precisa ser feito na primeira vez ou após alterações no código) e iniciar os containers.

Passo 1: Construir as Imagens (Build)

Primeiro, navegue até a pasta raiz do seu projeto (PBL1_Redes-card_game/) no terminal. Em seguida, execute o comando build. Ele lerá seu arquivo docker-compose.yml, encontrará as instruções de build para os serviços server, client e tests, e criará as respectivas imagens Docker.
Bash

docker-compose build

Passo 2: Iniciar o Servidor

Para iniciar o servidor, usamos o comando up. A flag -d (detached) faz com que o container do servidor rode em segundo plano, liberando seu terminal.
Bash

docker-compose up -d server

Neste momento, seu servidor de jogo está no ar e escutando na porta 8080, pronto para receber conexões.

2. Conectar os Containers e Serviços (Como Funciona)

A orquestração da comunicação é gerenciada inteiramente pelo Docker Compose, baseado no que foi definido no arquivo docker-compose.yml:

    Rede Virtual: Foi criada uma rede privada chamada cardgame-net. Todos os serviços (

server, client, tests) são conectados a esta rede, permitindo que eles se comuniquem uns com os outros usando seus nomes de serviço como se fossem nomes de domínio.

Resolução de Nomes: Dentro desta rede, o container do client pode encontrar o container do server simplesmente se conectando ao endereço server:8080. O Docker se encarrega de direcionar o tráfego para o IP interno correto do container do servidor.

Ordem de Inicialização: A diretiva depends_on: - server no serviço do client e tests garante que o Docker Compose sempre iniciará o container do server antes de iniciar qualquer um dos outros containers que precisam se conectar a ele.

3. Acessar o Jogo (Executar o Cliente)

Com o servidor rodando em segundo plano, você pode iniciar o cliente para começar a jogar. O cliente é um aplicativo de console interativo.

Execute o seguinte comando no seu terminal:
Bash

docker-compose run --rm client server

Análise do Comando:

    docker-compose run: Inicia uma nova instância do serviço especificado. É ideal para tarefas que você executa e que depois terminam, como o cliente ou os testes.

    --rm: Uma flag muito útil que remove automaticamente o container do cliente assim que você encerrar a sessão (digitando exit), mantendo seu sistema limpo.

    client: O nome do serviço que queremos executar, conforme definido no docker-compose.yml.

    server: Este é um argumento passado para o comando de inicialização do cliente. O seu userStart.go foi programado para usar este argumento como o endereço IP do servidor. Dentro da rede Docker, server é o nome de host do container do servidor.

Após executar este comando, você estará conectado ao terminal interativo do cliente e poderá usar os comandos do jogo (stock, pack, play, ping, etc.).

4. Monitoramento e Logs

Para verificar o que está acontecendo no servidor (conexões de jogadores, pareamentos, erros, etc.), você pode visualizar os logs do container do servidor em tempo real.

Abra um novo terminal (ou use o que você usou para iniciar o servidor, se não usou a flag -d) e execute:
Bash

docker-compose logs -f server

    logs: O comando para visualizar os logs.

    -f (follow): Mantém o terminal aberto, exibindo novas mensagens de log assim que elas são geradas pelo servidor.

Pressione Ctrl+C para parar de visualizar os logs.

5. Encerramento do Jogo

Quando você terminar de jogar e quiser desligar todo o ambiente, use o comando down.
Bash

docker-compose down

Este comando irá:

    Parar os containers que estiverem em execução (como o server).

    Remover os containers criados.

    Remover a rede virtual (cardgame-net) que foi criada.

Isso garante que todos os recursos utilizados pelo Docker para o seu jogo sejam completamente liberados da sua máquina.
