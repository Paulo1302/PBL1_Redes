// File: server/game_server.go
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"PBL1_Redes-card_game/game"
)

// Client representa um jogador conectado e seu estado completo no servidor.
// Gerencia não apenas a conexão, mas também o estado do jogador no jogo (ocioso, na fila, em jogo).
type Client struct {
	Name       string            // Nome do jogador, fornecido durante o registro.
	Connection net.Conn          // A conexão TCP subjacente com o cliente.
	IP         string            // Endereço IP do cliente, usado como identificador único.
	Stock      map[game.Card]int // O inventário de cartas do jogador.
	PlayedCard game.Card         // A carta jogada na rodada atual.
	Encoder    *json.Encoder     // Encoder JSON para enviar mensagens para este cliente.
	Decoder    *json.Decoder     // Decoder JSON para receber mensagens deste cliente.
	MoveChan   chan game.Card    // Canal para comunicar a jogada de um cliente para a goroutine do jogo.
	stateMutex sync.RWMutex      // Mutex para garantir o acesso seguro ao campo State.
	State      string            // Estado atual do cliente: "Idle", "InQueue", "InGame".
}

// Message define a estrutura padrão para toda a comunicação entre cliente e servidor.
type Message struct {
	Action string          `json:"action"` // Ação a ser executada (ex: "register", "play").
	Data   json.RawMessage `json:"data"`   // Os dados associados à ação, em formato JSON bruto.
}

// Payloads de comunicação para diferentes tipos de mensagens.
// Eles estruturam os dados dentro do campo 'Data' da struct Message.

type ResponseMessage struct {
	Content string `json:"content"`
}
type RegisterPayload struct {
	Name  string            `json:"name"`
	Stock map[game.Card]int `json:"stock"`
}
type GameOverPayload struct {
	Content    string            `json:"content"`
	FinalStock map[game.Card]int `json:"final_stock"`
}
type OpenPackPayload struct {
	Content  string            `json:"content"`
	NewCards []game.Card       `json:"new_cards"`
	NewStock map[game.Card]int `json:"new_stock"`
}
type RoundResultPayload struct {
	Result     string    `json:"result"`
	Winner     string    `json:"winner,omitempty"`
	Loser      string    `json:"loser,omitempty"`
	WinnerCard game.Card `json:"winner_card,omitempty"`
	LoserCard  game.Card `json:"loser_card,omitempty"`
	DrawCard   game.Card `json:"draw_card,omitempty"`
}

// WaitingQueueManager gerencia a fila de espera de jogadores de forma concorrente.
type WaitingQueueManager struct {
	locker       sync.RWMutex
	WaitingQueue []*Client
}

// PairedClientsManager gerencia o mapa de clientes que estão atualmente em uma partida.
type PairedClientsManager struct {
	locker        sync.RWMutex
	PairedClients map[string]string // Mapeia o IP de um jogador ao IP de seu oponente.
}

// GlobalCardStock gerencia o estoque de cartas do servidor, usado para distribuir pacotes.
type GlobalCardStock struct {
	mu    sync.Mutex
	Cards map[game.Card]int
}

// Implementação da interface game.Player para a struct Client.
// Isso permite que a lógica de jogo genérica opere sobre a struct Client.
func (c *Client) GetName() string                { return c.Name }
func (c *Client) GetStock() map[game.Card]int   { return c.Stock }
func (c *Client) GetPlayedCard() game.Card     { return c.PlayedCard }
func (c *Client) SetPlayedCard(card game.Card) { c.PlayedCard = card }
func (c *Client) RemoveCard(card game.Card) {
	if c.Stock[card] > 0 {
		c.Stock[card]--
		if c.Stock[card] == 0 {
			delete(c.Stock, card)
		}
	}
}
func (c *Client) AddCard(card game.Card) { c.Stock[card]++ }

// SetState e GetState permitem a alteração e leitura segura do estado do cliente.
func (c *Client) SetState(state string) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.State = state
}
func (c *Client) GetState() string {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.State
}

// Métodos dos gerenciadores para manipulação segura de estado concorrente.
func (m *PairedClientsManager) MatchAdder(c1 *Client, c2 *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()
	m.PairedClients[c1.IP] = c2.IP
	m.PairedClients[c2.IP] = c1.IP
	fmt.Println("Pairs created successfully.")
}
func (m *PairedClientsManager) MatchRemover(client *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()
	if pairID, ok := m.PairedClients[client.IP]; ok {
		delete(m.PairedClients, client.IP)
		delete(m.PairedClients, pairID)
		fmt.Printf("Client pair removed successfully: %s and %s.\n", client.IP, pairID)
	} else {
		// Esta mensagem é normal se um cliente desconectar antes de ser pareado.
		fmt.Printf("Client %s not found in the pairs map.\n", client.IP)
	}
}
func (m *WaitingQueueManager) WQueueAdder(client *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()
	m.WaitingQueue = append(m.WaitingQueue, client)
}
func (m *WaitingQueueManager) WQueueRemover(client *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()
	for i, c := range m.WaitingQueue {
		if c.IP == client.IP {
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			return
		}
	}
}

// Variáveis globais que mantêm o estado do servidor.
var (
	// Gerenciador da fila de espera.
	waitingQueueManager = &WaitingQueueManager{WaitingQueue: []*Client{}}
	// Gerenciador de clientes em partida.
	pairedClientsManager = &PairedClientsManager{PairedClients: make(map[string]string)}
	// Estoque de cartas do servidor para a funcionalidade "open_pack".
	globalStock = &GlobalCardStock{
		Cards: map[game.Card]int{
			game.Rock: 300, game.Paper: 300, game.Scissors: 300,
		},
	}
)

// Start inicializa o listener TCP do servidor e retorna uma função de 'shutdown'
// para fechar o servidor de forma graciosa.
func Start() (shutdown func()) {
	fmt.Println("Game server starting on port 8080...")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(fmt.Sprintf("Error starting server: %v", err))
	}

	// Goroutine principal que aceita novas conexões.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Erro geralmente ocorre quando o listener.Close() é chamado.
				fmt.Println("Listener closed, shutting down accept loop.")
				return
			}
			// Cada conexão é gerenciada em sua própria goroutine.
			go handleClientConnection(conn)
		}
	}()

	// Retorna a função que, quando chamada, fechará o listener.
	return func() {
		fmt.Println("Shutting down server...")
		listener.Close()
	}
}

// handleClientConnection é o ponto de entrada para um cliente recém-conectado.
// Ele gerencia o ciclo de vida completo da conexão do cliente.
func handleClientConnection(conn net.Conn) {
	// Garante que a conexão seja fechada quando esta função retornar.
	defer conn.Close()

	// O primeiro passo é sempre registrar o cliente.
	client, err := handleRegister(conn)
	if err != nil {
		fmt.Printf("Failed to register client %s: %v\n", conn.RemoteAddr().String(), err)
		return
	}
	fmt.Printf("Client '%s' (%s) registered successfully.\n", client.Name, client.IP)

	// Loop principal de leitura de mensagens do cliente.
	for {
		var msg Message
		err := client.Decoder.Decode(&msg)
		if err != nil {
			// Um erro aqui geralmente significa que o cliente desconectou (EOF).
			fmt.Printf("Client %s disconnected while in state '%s': %v\n", client.IP, client.GetState(), err)
			close(client.MoveChan) // Fecha o canal de jogadas para desbloquear a goroutine do jogo.
			// Remove o cliente de qualquer estado ativo para limpeza.
			waitingQueueManager.WQueueRemover(client)
			pairedClientsManager.MatchRemover(client)
			return // Encerra a goroutine e o defer conn.Close() é executado.
		}

		// Roteia a ação recebida para a função de tratamento apropriada.
		switch msg.Action {
		case "ping":
			handlePing(client)
		case "play": // 'play' é a ação de fazer uma jogada em um jogo.
			handlePlay(client, msg)
		case "queue_for_match": // 'queue_for_match' é a ação de entrar na fila.
			handleQueueForMatch(client)
		case "open_pack":
			handleOpenPack(client)
		default:
			sendErrorMessage(client.Encoder, fmt.Sprintf("Unknown action: %s", msg.Action))
		}
	}
}

// handleRegister processa a primeira mensagem de um cliente, que deve ser de registro.
func handleRegister(conn net.Conn) (*Client, error) {
	decoder := json.NewDecoder(conn)
	var msg Message
	// Define um timeout para o registro para evitar conexões penduradas.
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("could not read register message: %w", err)
	}
	// Limpa o timeout após a leitura bem-sucedida.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return nil, err
	}

	if msg.Action != "register" {
		return nil, errors.New("expected 'register' action as first message")
	}

	var payload RegisterPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return nil, fmt.Errorf("invalid register payload: %w", err)
	}

	// Cria e retorna a struct Client com os dados iniciais.
	client := &Client{
		Name:       payload.Name,
		Stock:      payload.Stock,
		Connection: conn,
		IP:         conn.RemoteAddr().String(),
		Encoder:    json.NewEncoder(conn),
		Decoder:    decoder,
		MoveChan:   make(chan game.Card),
		State:      "Idle", // Estado inicial de todo cliente.
	}
	return client, nil
}

// handleQueueForMatch processa a solicitação de um cliente para entrar na fila de matchmaking.
func handleQueueForMatch(client *Client) {
	// Validação de estado: o cliente só pode entrar na fila se estiver ocioso.
	if client.GetState() != "Idle" {
		sendErrorMessage(client.Encoder, "You cannot enter the queue while already in a queue or game.")
		return
	}
	// Validação de regras: o cliente precisa ter cartas para jogar.
	if len(client.GetStock()) == 0 {
		sendErrorMessage(client.Encoder, "You have no cards left! Open a pack to get more cards before playing.")
		return
	}

	client.SetState("InQueue")
	fmt.Printf("Client %s requested to enter the matchmaking queue.\n", client.IP)
	waitingQueueManager.WQueueAdder(client)

	confirmMsg, _ := json.Marshal(ResponseMessage{Content: "You are now in the matchmaking queue. Waiting for an opponent..."})
	sendActionMessage(client.Encoder, "queue_success", confirmMsg)
	fmt.Printf("Client %s added to the queue. Queue now has %d client(s).\n", client.IP, len(waitingQueueManager.WaitingQueue))

	// Tenta formar uma partida sempre que um novo jogador entra na fila.
	tryMatch()
}

// handleOpenPack gerencia a lógica de um jogador abrir um pacote de cartas.
func handleOpenPack(client *Client) {
	// Validação de estado: só é possível abrir pacotes quando ocioso.
	if client.GetState() != "Idle" {
		sendErrorMessage(client.Encoder, "You can only open packs when you are not in a queue or in a game.")
		return
	}
	// Bloqueia o estoque global para garantir que a operação seja atômica.
	globalStock.mu.Lock()
	defer globalStock.mu.Unlock()

	// Cria uma lista de todas as cartas disponíveis no servidor.
	availableCards := make([]game.Card, 0)
	for card, quantity := range globalStock.Cards {
		for i := 0; i < quantity; i++ {
			availableCards = append(availableCards, card)
		}
	}

	if len(availableCards) < 3 {
		sendErrorMessage(client.Encoder, "Sorry, there are not enough cards available in the server to open a new pack.")
		return
	}

	// Sorteia 3 cartas aleatórias da lista de cartas disponíveis.
	receivedCards := make([]game.Card, 0, 3)
	for i := 0; i < 3; i++ {
		randomIndex := rand.Intn(len(availableCards))
		chosenCard := availableCards[randomIndex]

		receivedCards = append(receivedCards, chosenCard)
		client.AddCard(chosenCard)      // Adiciona a carta ao inventário do cliente.
		globalStock.Cards[chosenCard]-- // Remove a carta do estoque global.

		// Remove a carta sorteada da lista para evitar que seja sorteada novamente.
		availableCards = append(availableCards[:randomIndex], availableCards[randomIndex+1:]...)
	}

	// Envia a resposta de sucesso para o cliente com as novas cartas e o inventário atualizado.
	payload := OpenPackPayload{
		Content:  "You opened a pack and received 3 new cards!",
		NewCards: receivedCards,
		NewStock: client.GetStock(),
	}
	responseData, _ := json.Marshal(payload)
	sendActionMessage(client.Encoder, "open_pack_success", responseData)
}

// handlePlay processa uma jogada enviada por um cliente durante uma partida.
func handlePlay(client *Client, msg Message) {
	// Validação de estado: o cliente deve estar em um jogo.
	if client.GetState() != "InGame" {
		sendErrorMessage(client.Encoder, "You can only make a move when you are in a game.")
		return
	}

	var payload struct {
		ChosenCard game.Card `json:"chosen_card"`
	}
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		sendErrorMessage(client.Encoder, "Invalid move data.")
		return
	}
	// Validação de regra: o cliente deve possuir a carta que está tentando jogar.
	if stock, ok := client.GetStock()[payload.ChosenCard]; !ok || stock <= 0 {
		sendErrorMessage(client.Encoder, fmt.Sprintf("You do not have the card '%s'.", payload.ChosenCard))
		return
	}

	// Envia a jogada através do canal para a goroutine 'startGame' que está gerenciando a partida.
	client.MoveChan <- payload.ChosenCard
	fmt.Printf("Move received from %s: %s\n", client.IP, payload.ChosenCard)
}

// tryMatch é a lógica central de matchmaking.
func tryMatch() {
	// Bloqueia a fila de espera para evitar condições de corrida.
	waitingQueueManager.locker.Lock()
	defer waitingQueueManager.locker.Unlock()

	// Enquanto houver pelo menos 2 jogadores na fila, forma uma partida.
	for len(waitingQueueManager.WaitingQueue) >= 2 {
		c1 := waitingQueueManager.WaitingQueue[0]
		c2 := waitingQueueManager.WaitingQueue[1]
		// Remove os jogadores pareados da fila de espera.
		waitingQueueManager.WaitingQueue = waitingQueueManager.WaitingQueue[2:]

		c1.SetState("InGame")
		c2.SetState("InGame")

		fmt.Printf("Clients paired: %s vs %s\n", c1.IP, c2.IP)
		pairedClientsManager.MatchAdder(c1, c2)
		pairedMsg, _ := json.Marshal(ResponseMessage{Content: "You have been paired! The game is starting..."})
		sendActionMessage(c1.Encoder, "matched", pairedMsg)
		sendActionMessage(c2.Encoder, "matched", pairedMsg)

		// Inicia a partida em uma nova goroutine para não bloquear o matchmaking.
		go startGame(c1, c2)
	}
}

// handlePing responde a uma mensagem de ping, útil para verificar a latência.
func handlePing(client *Client) {
	fmt.Printf("Received ping from %s, sending pong.\n", client.IP)
	pongMsg, _ := json.Marshal(ResponseMessage{Content: "pong response"})
	sendActionMessage(client.Encoder, "pong", pongMsg)
}

// startGame gerencia o fluxo de uma partida de rodada única entre dois clientes.
// É executado em sua própria goroutine.
func startGame(c1, c2 *Client) {
	fmt.Printf("Starting single-round match between %s and %s\n", c1.IP, c2.IP)
	const moveTimeout = 30 * time.Second

	// 'defer' garante que o estado seja limpo (reset para "Idle") e os pares
	// sejam removidos no final da partida, independentemente de como ela termina.
	defer func() {
		c1.SetState("Idle")
		c2.SetState("Idle")
		pairedClientsManager.MatchRemover(c1)
		fmt.Printf("Match between %s and %s ended. States reset to Idle.\n", c1.IP, c2.IP)
	}()

	// Validação de segurança: verifica se os jogadores ainda têm cartas.
	if len(c1.GetStock()) == 0 || len(c2.GetStock()) == 0 {
		endGameByCardCount(c1, c2)
		return
	}

	// Envia a mensagem para os clientes solicitando uma jogada.
	promptMsg, _ := json.Marshal(ResponseMessage{Content: "Your turn to play! This is a single-round match."})
	sendActionMessage(c1.Encoder, "game_prompt_play", promptMsg)
	sendActionMessage(c2.Encoder, "game_prompt_play", promptMsg)

	var move1, move2 game.Card
	var firstPlayer, secondPlayer *Client

	// Passo 1: Espera pela jogada do PRIMEIRO jogador que se mover.
	// O select aguarda em múltiplos canais simultaneamente.
	select {
	case move, ok := <-c1.MoveChan:
		if !ok { // Canal fechado indica que o cliente desconectou.
			endGameByDisconnect(c2, c1) // c2 vence.
			return
		}
		move1 = move
		firstPlayer, secondPlayer = c1, c2

	case move, ok := <-c2.MoveChan:
		if !ok {
			endGameByDisconnect(c1, c2) // c1 vence.
			return
		}
		move1 = move
		firstPlayer, secondPlayer = c2, c1

	case <-time.After(moveTimeout * 2): // Timeout se nenhum jogador se mover.
		drawMsg, _ := json.Marshal(GameOverPayload{Content: "Match ended due to inactivity from both players.", FinalStock: c1.GetStock()})
		drawMsg2, _ := json.Marshal(GameOverPayload{Content: "Match ended due to inactivity from both players.", FinalStock: c2.GetStock()})
		sendActionMessage(c1.Encoder, "game_over", drawMsg)
		sendActionMessage(c2.Encoder, "game_over", drawMsg2)
		return
	}

	// Passo 2: Notifica o SEGUNDO jogador que o oponente já jogou.
	waitMsg, _ := json.Marshal(ResponseMessage{Content: "Your opponent has made a move. Waiting for you..."})
	sendActionMessage(secondPlayer.Encoder, "game_status_update", waitMsg)

	// Passo 3: Espera pela jogada do SEGUNDO jogador, com um timeout menor.
	select {
	case move, ok := <-secondPlayer.MoveChan:
		if !ok {
			endGameByDisconnect(firstPlayer, secondPlayer) // O primeiro jogador vence.
			return
		}
		move2 = move

	case <-time.After(moveTimeout): // Timeout se o segundo jogador demorar demais.
		endGameByTimeout(firstPlayer, secondPlayer) // O primeiro jogador vence.
		return
	}

	// Passo 4: Ambas as jogadas foram recebidas. Processa o resultado.
	firstPlayer.SetPlayedCard(move1)
	secondPlayer.SetPlayedCard(move2)
	firstPlayer.RemoveCard(move1)
	secondPlayer.RemoveCard(move2)

	winner, loser := game.DetermineWinner(firstPlayer, secondPlayer)

	if winner == nil {
		// Cenário de Empate.
		drawPayload := GameOverPayload{
			Content:    "Game Over! The round was a draw.",
			FinalStock: firstPlayer.GetStock(),
		}
		drawPayload2 := GameOverPayload{
			Content:    "Game Over! The round was a draw.",
			FinalStock: secondPlayer.GetStock(),
		}
		drawMsg, _ := json.Marshal(drawPayload)
		drawMsg2, _ := json.Marshal(drawPayload2)
		sendActionMessage(firstPlayer.Encoder, "game_over", drawMsg)
		sendActionMessage(secondPlayer.Encoder, "game_over", drawMsg2)
		fmt.Printf("Match ended in a draw between %s and %s.\n", c1.IP, c2.IP)
	} else {
		// Cenário de Vitória/Derrota.
		game.SwapCards(winner, loser) // Realiza a troca de cartas.
		fmt.Printf("Round winner: %s. Cards swapped.\n", winner.GetName())

		winMsg, _ := json.Marshal(GameOverPayload{Content: "You won the round!", FinalStock: winner.GetStock()})
		loseMsg, _ := json.Marshal(GameOverPayload{Content: "You lost the round!", FinalStock: loser.GetStock()})

		// Envia as mensagens de game over para o vencedor e o perdedor.
		// É necessária a asserção de tipo para acessar o campo Encoder.
		sendActionMessage(winner.(*Client).Encoder, "game_over", winMsg)
		sendActionMessage(loser.(*Client).Encoder, "game_over", loseMsg)
	}
}

// Funções Auxiliares para diferentes cenários de Fim de Jogo.

// endGameByTimeout é chamado quando o segundo jogador não joga a tempo.
func endGameByTimeout(winner, loser *Client) {
	fmt.Printf("Match ended by timeout. Winner: %s, Loser: %s\n", winner.IP, loser.IP)

	winMsg, _ := json.Marshal(GameOverPayload{Content: "You won! Your opponent failed to make a move in time.", FinalStock: winner.GetStock()})
	loseMsg, _ := json.Marshal(GameOverPayload{Content: "You lost! You failed to make a move in time.", FinalStock: loser.GetStock()})

	sendActionMessage(winner.Encoder, "game_over", winMsg)
	sendActionMessage(loser.Encoder, "game_over", loseMsg)
}

// endGameByDisconnect é chamado quando um jogador desconecta no meio da partida.
func endGameByDisconnect(winner, loser *Client) {
	fmt.Printf("A player disconnected. Ending match. Winner: %s\n", winner.IP)

	gameOverMsg, _ := json.Marshal(GameOverPayload{Content: "Your opponent disconnected. You win!", FinalStock: winner.GetStock()})
	// Apenas o vencedor recebe a mensagem, pois o perdedor já está desconectado.
	sendActionMessage(winner.Encoder, "game_over", gameOverMsg)
}

// endGameByCardCount é chamado se o jogo iniciar com um jogador sem cartas.
func endGameByCardCount(c1, c2 *Client) {
	var finalWinner, finalLoser *Client
	if len(c1.GetStock()) > len(c2.GetStock()) {
		finalWinner, finalLoser = c1, c2
	} else {
		finalWinner, finalLoser = c2, c1
	}

	gameOverWinMsg, _ := json.Marshal(GameOverPayload{Content: fmt.Sprintf("Game Over! You win! Cards left: %d.", len(finalWinner.GetStock())), FinalStock: finalWinner.GetStock()})
	gameOverLoseMsg, _ := json.Marshal(GameOverPayload{Content: fmt.Sprintf("Game Over! You lose! Cards left: %d.", len(finalLoser.GetStock())), FinalStock: finalLoser.GetStock()})

	sendActionMessage(finalWinner.Encoder, "game_over", gameOverWinMsg)
	sendActionMessage(finalLoser.Encoder, "game_over", gameOverLoseMsg)

	fmt.Printf("Match finished by card count between %s and %s. Winner: %s\n", c1.IP, c2.IP, finalWinner.IP)
}

// Funções utilitárias para padronizar o envio de mensagens.

// sendActionMessage envia uma mensagem de ação padrão.
func sendActionMessage(encoder *json.Encoder, action string, data json.RawMessage) {
	finalMsg := Message{
		Action: action,
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}

// sendErrorMessage envia uma mensagem de erro padronizada.
func sendErrorMessage(encoder *json.Encoder, errorMsg string) {
	payload := map[string]string{"error": errorMsg}
	data, _ := json.Marshal(payload)
	finalMsg := Message{
		Action: "error",
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}