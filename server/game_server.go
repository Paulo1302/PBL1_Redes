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

// Client representa um cliente conectado, com gerenciamento de estado.
type Client struct {
	Name       string
	Connection net.Conn
	IP         string
	Stock      map[game.Card]int
	PlayedCard game.Card
	Encoder    *json.Encoder
	Decoder    *json.Decoder
	MoveChan   chan game.Card
	stateMutex sync.RWMutex
	State      string // Estado: "Idle", "InQueue", "InGame"
}

// Payloads, Managers e a implementação da interface Player (sem alterações)
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}
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

type WaitingQueueManager struct {
	locker       sync.RWMutex
	WaitingQueue []*Client
}
type PairedClientsManager struct {
	locker        sync.RWMutex
	PairedClients map[string]string
}
type GlobalCardStock struct {
	mu    sync.Mutex
	Cards map[game.Card]int
}

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

var (
	waitingQueueManager  = &WaitingQueueManager{WaitingQueue: []*Client{}}
	pairedClientsManager = &PairedClientsManager{PairedClients: make(map[string]string)}
	globalStock          = &GlobalCardStock{
		Cards: map[game.Card]int{
			game.Rock: 300, game.Paper: 300, game.Scissors: 300,
		},
	}
)

// Start inicializa o servidor e retorna uma função para desligá-lo.
func Start() (shutdown func()) {
	fmt.Println("Game server starting on port 8080...")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(fmt.Sprintf("Error starting server: %v", err))
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				fmt.Println("Listener closed, shutting down accept loop.")
				return
			}
			go handleClientConnection(conn)
		}
	}()

	return func() {
		fmt.Println("Shutting down server...")
		listener.Close()
	}
}

// handleClientConnection agora é um loop perpétuo que gerencia a conexão.
func handleClientConnection(conn net.Conn) {
	// defer conn.Close() agora gerencia o ciclo de vida completo da conexão.
	defer conn.Close()

	client, err := handleRegister(conn)
	if err != nil {
		fmt.Printf("Failed to register client %s: %v\n", conn.RemoteAddr().String(), err)
		return
	}
	fmt.Printf("Client '%s' (%s) registered successfully.\n", client.Name, client.IP)

	// O loop agora é infinito. Apenas uma desconexão do cliente o interrompe.
	for {
		var msg Message
		err := client.Decoder.Decode(&msg)
		if err != nil {
			fmt.Printf("Client %s disconnected while in state '%s': %v\n", client.IP, client.GetState(), err)
			close(client.MoveChan) // Sinaliza para a goroutine do jogo que o jogador saiu
			waitingQueueManager.WQueueRemover(client)
			pairedClientsManager.MatchRemover(client)
			return // Encerra a goroutine e o defer conn.Close() é chamado.
		}

		// O roteamento de ações agora é feito com base no estado do cliente.
		switch msg.Action {
		case "ping":
			handlePing(client)
		case "play": // 'play' agora é a ação de fazer uma jogada
			handlePlay(client, msg)
		case "queue_for_match":
			handleQueueForMatch(client)
		case "open_pack":
			handleOpenPack(client)
		default:
			sendErrorMessage(client.Encoder, fmt.Sprintf("Unknown action: %s", msg.Action))
		}
	}
}

// handleRegister permanece o mesmo, mas a lógica de estado do client é crucial.
func handleRegister(conn net.Conn) (*Client, error) {
	decoder := json.NewDecoder(conn)
	var msg Message
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("could not read register message: %w", err)
	}
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
	client := &Client{
		Name:       payload.Name,
		Stock:      payload.Stock,
		Connection: conn,
		IP:         conn.RemoteAddr().String(),
		Encoder:    json.NewEncoder(conn),
		Decoder:    decoder,
		MoveChan:   make(chan game.Card),
		State:      "Idle",
	}
	return client, nil
}

// Handlers agora validam o estado do jogador antes de executar a ação.

func handleQueueForMatch(client *Client) {
	if client.GetState() != "Idle" {
		sendErrorMessage(client.Encoder, "You cannot enter the queue while already in a queue or game.")
		return
	}
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
	tryMatch()
}

func handleOpenPack(client *Client) {
	if client.GetState() != "Idle" {
		sendErrorMessage(client.Encoder, "You can only open packs when you are not in a queue or in a game.")
		return
	}
	globalStock.mu.Lock()
	defer globalStock.mu.Unlock()

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

	receivedCards := make([]game.Card, 0, 3)
	for i := 0; i < 3; i++ {
		randomIndex := rand.Intn(len(availableCards))
		chosenCard := availableCards[randomIndex]
		receivedCards = append(receivedCards, chosenCard)
		client.AddCard(chosenCard)
		globalStock.Cards[chosenCard]--
		availableCards = append(availableCards[:randomIndex], availableCards[randomIndex+1:]...)
	}

	payload := OpenPackPayload{
		Content:  "You opened a pack and received 3 new cards!",
		NewCards: receivedCards,
		NewStock: client.GetStock(),
	}
	responseData, _ := json.Marshal(payload)
	sendActionMessage(client.Encoder, "open_pack_success", responseData)
}

func handlePlay(client *Client, msg Message) {
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
	if stock, ok := client.GetStock()[payload.ChosenCard]; !ok || stock <= 0 {
		sendErrorMessage(client.Encoder, fmt.Sprintf("You do not have the card '%s'.", payload.ChosenCard))
		return
	}
	client.MoveChan <- payload.ChosenCard
	fmt.Printf("Move received from %s: %s\n", client.IP, payload.ChosenCard)
}

func tryMatch() {
	waitingQueueManager.locker.Lock()
	defer waitingQueueManager.locker.Unlock()

	for len(waitingQueueManager.WaitingQueue) >= 2 {
		c1 := waitingQueueManager.WaitingQueue[0]
		c2 := waitingQueueManager.WaitingQueue[1]
		waitingQueueManager.WaitingQueue = waitingQueueManager.WaitingQueue[2:]

		c1.SetState("InGame")
		c2.SetState("InGame")

		fmt.Printf("Clients paired: %s vs %s\n", c1.IP, c2.IP)
		pairedClientsManager.MatchAdder(c1, c2)
		pairedMsg, _ := json.Marshal(ResponseMessage{Content: "You have been paired! The game is starting..."})
		sendActionMessage(c1.Encoder, "matched", pairedMsg)
		sendActionMessage(c2.Encoder, "matched", pairedMsg)
		go startGame(c1, c2)
	}
}

func handlePing(client *Client) {
	fmt.Printf("Received ping from %s, sending pong.\n", client.IP)
	pongMsg, _ := json.Marshal(ResponseMessage{Content: "pong response"})
	sendActionMessage(client.Encoder, "pong", pongMsg)
}

// startGame gerencia o fluxo de uma partida de rodada única.
func startGame(c1, c2 *Client) {
	fmt.Printf("Starting single-round match between %s and %s\n", c1.IP, c2.IP)
	const moveTimeout = 30 * time.Second

	// 'defer' garante que o estado seja limpo e os pares removidos no final da partida.
	defer func() {
		c1.SetState("Idle")
		c2.SetState("Idle")
		pairedClientsManager.MatchRemover(c1)
		fmt.Printf("Match between %s and %s ended. States reset to Idle.\n", c1.IP, c2.IP)
	}()

	// Verifica se os jogadores têm cartas antes de começar. Se não, encerra.
	if len(c1.GetStock()) == 0 || len(c2.GetStock()) == 0 {
		endGameByCardCount(c1, c2)
		return
	}

	promptMsg, _ := json.Marshal(ResponseMessage{Content: "Your turn to play! This is a single-round match."})
	sendActionMessage(c1.Encoder, "game_prompt_play", promptMsg)
	sendActionMessage(c2.Encoder, "game_prompt_play", promptMsg)

	var move1, move2 game.Card
	var firstPlayer, secondPlayer *Client

	// 1. Espera pela jogada do PRIMEIRO jogador que se mover.
	select {
	case move, ok := <-c1.MoveChan:
		if !ok {
			endGameByDisconnect(c2, c1) // c2 vence porque c1 desconectou
			return
		}
		move1 = move
		firstPlayer, secondPlayer = c1, c2

	case move, ok := <-c2.MoveChan:
		if !ok {
			endGameByDisconnect(c1, c2) // c1 vence porque c2 desconectou
			return
		}
		move1 = move
		firstPlayer, secondPlayer = c2, c1

	case <-time.After(moveTimeout * 2):
		drawMsg, _ := json.Marshal(GameOverPayload{Content: "Match ended due to inactivity from both players.", FinalStock: c1.GetStock()})
		drawMsg2, _ := json.Marshal(GameOverPayload{Content: "Match ended due to inactivity from both players.", FinalStock: c2.GetStock()})
		sendActionMessage(c1.Encoder, "game_over", drawMsg)
		sendActionMessage(c2.Encoder, "game_over", drawMsg2)
		return
	}

	// 2. Notifica o SEGUNDO jogador que o oponente já jogou.
	waitMsg, _ := json.Marshal(ResponseMessage{Content: "Your opponent has made a move. Waiting for you..."})
	sendActionMessage(secondPlayer.Encoder, "game_status_update", waitMsg)

	// 3. Espera pela jogada do SEGUNDO jogador, agora com um timeout.
	select {
	case move, ok := <-secondPlayer.MoveChan:
		if !ok {
			endGameByDisconnect(firstPlayer, secondPlayer) // O primeiro jogador vence
			return
		}
		move2 = move

	case <-time.After(moveTimeout):
		endGameByTimeout(firstPlayer, secondPlayer) // O primeiro jogador vence
		return
	}

	// 4. Ambas as jogadas foram recebidas, processa a rodada e finaliza o jogo.
	firstPlayer.SetPlayedCard(move1)
	secondPlayer.SetPlayedCard(move2)
	firstPlayer.RemoveCard(move1)
	secondPlayer.RemoveCard(move2)

	winner, loser := game.DetermineWinner(firstPlayer, secondPlayer)

	if winner == nil {
		// Empate: Apenas informa o fim do jogo.
		drawPayload := GameOverPayload{
			Content:    "Game Over! The round was a draw.",
			FinalStock: firstPlayer.GetStock(), // Estoque final do primeiro jogador
		}
		drawPayload2 := GameOverPayload{
			Content:    "Game Over! The round was a draw.",
			FinalStock: secondPlayer.GetStock(), // Estoque final do segundo jogador
		}
		drawMsg, _ := json.Marshal(drawPayload)
		drawMsg2, _ := json.Marshal(drawPayload2)
		sendActionMessage(firstPlayer.Encoder, "game_over", drawMsg)
		sendActionMessage(secondPlayer.Encoder, "game_over", drawMsg2)
		fmt.Printf("Match ended in a draw between %s and %s.\n", c1.IP, c2.IP)
	} else {
		// Houve um vencedor: distribui as cartas e informa o fim do jogo.
		game.SwapCards(winner, loser)
		fmt.Printf("Round winner: %s. Cards swapped.\n", winner.GetName())

		winMsg, _ := json.Marshal(GameOverPayload{Content: "You won the round!", FinalStock: winner.GetStock()})
		loseMsg, _ := json.Marshal(GameOverPayload{Content: "You lost the round!", FinalStock: loser.GetStock()})

		// CORREÇÃO: Faz a asserção de tipo de game.Player para *Client para acessar o Encoder.
		sendActionMessage(winner.(*Client).Encoder, "game_over", winMsg)
		sendActionMessage(loser.(*Client).Encoder, "game_over", loseMsg)
	}
}

func endGameByTimeout(winner, loser *Client) {
	fmt.Printf("Match ended by timeout. Winner: %s, Loser: %s\n", winner.IP, loser.IP)
	
	winMsg, _ := json.Marshal(GameOverPayload{Content: "You won! Your opponent failed to make a move in time.", FinalStock: winner.GetStock()})
	loseMsg, _ := json.Marshal(GameOverPayload{Content: "You lost! You failed to make a move in time.", FinalStock: loser.GetStock()})

	sendActionMessage(winner.Encoder, "game_over", winMsg)
	sendActionMessage(loser.Encoder, "game_over", loseMsg)
}

// Funções Auxiliares de Fim de Jogo
func endGameByDisconnect(winner, loser *Client) {
	fmt.Printf("A player disconnected. Ending match. Winner: %s\n", winner.IP)

	gameOverMsg, _ := json.Marshal(GameOverPayload{Content: "Your opponent disconnected. You win!", FinalStock: winner.GetStock()})
	sendActionMessage(winner.Encoder, "game_over", gameOverMsg)
}

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

// Funções auxiliares de envio de mensagem.
func sendActionMessage(encoder *json.Encoder, action string, data json.RawMessage) {
	finalMsg := Message{
		Action: action,
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}
func sendErrorMessage(encoder *json.Encoder, errorMsg string) {
	payload := map[string]string{"error": errorMsg}
	data, _ := json.Marshal(payload)
	finalMsg := Message{
		Action: "error",
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}