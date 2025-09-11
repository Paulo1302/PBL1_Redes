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

	"PBL1_Redes/game"
)

// Client representa um cliente conectado. O Stock agora é preenchido pelo cliente.
type Client struct {
	Name       string
	Connection net.Conn
	IP         string
	Stock      map[game.Card]int
	PlayedCard game.Card
	Encoder    *json.Encoder
	Decoder    *json.Decoder
	MoveChan   chan game.Card
}

// Payloads para comunicação
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
// OpenPackPayload é a resposta estruturada ao abrir um pacote
type OpenPackPayload struct {
	Content  string            `json:"content"`
	NewCards []game.Card       `json:"new_cards"`
	NewStock map[game.Card]int `json:"new_stock"`
}

type WaitingQueueManager struct {
	locker       sync.RWMutex
	WaitingQueue []*Client
}
type PairedClientsManager struct {
	locker        sync.RWMutex
	PairedClients map[string]string
}

// GlobalCardStock armazena o baralho central de cartas disponíveis do servidor.
// É protegido por um mutex para lidar com o acesso simultâneo de forma segura.
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
	// Inicializa o estoque global com um número finito de cartas
	globalStock = &GlobalCardStock{
		Cards: map[game.Card]int{
			game.Rock:    20,
			game.Paper:   20,
			game.Scissors: 20,
		},
	}
)

// File: server/game_server.go

// Start initializes the game server and returns a function to shut it down.
func Start() (shutdown func()) {
	fmt.Println("Game server starting on port 8080...")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		// Em um teste, não podemos nos recuperar disso, então causamos pânico.
		// Em produção real, um log fatal seria mais apropriado.
		panic(fmt.Sprintf("Error starting server: %v", err))
	}

	// A goroutine anônima executa o loop de aceitação de conexões.
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				// Quando listener.Close() é chamado, Accept() retorna um erro.
				// Este é o sinal para parar de aceitar conexões.
				fmt.Println("Listener closed, shutting down accept loop.")
				return
			}
			go handleClientConnection(conn)
		}
	}()

	// Retorna uma função que, quando chamada, fechará o listener.
	return func() {
		fmt.Println("Shutting down server...")
		listener.Close()
	}
}

func handleClientConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)

	client, err := handleRegister(conn)
	if err != nil {
		fmt.Printf("Failed to register client %s: %v\n", conn.RemoteAddr().String(), err)
		return
	}
	fmt.Printf("Client '%s' (%s) registered successfully.\n", client.Name, client.IP)

	for {
		var msg Message
		err := decoder.Decode(&msg)
		if err != nil {
			fmt.Printf("Client %s disconnected: %v\n", client.IP, err)
			close(client.MoveChan)
			waitingQueueManager.WQueueRemover(client)
			pairedClientsManager.MatchRemover(client)
			return
		}

		switch msg.Action {
		case "ping":
			handlePing(client)
		case "play":
			handlePlay(client, msg)
		case "queue_for_match":
			handleQueueForMatch(client)
		case "open_pack": // NOVA AÇÃO
			handleOpenPack(client)
		default:
			sendErrorMessage(client.Encoder, "Unknown action.")
		}
	}
}

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
	}
	return client, nil
}

func handleQueueForMatch(client *Client) {
	fmt.Printf("Client %s requested to enter the matchmaking queue.\n", client.IP)
	waitingQueueManager.WQueueAdder(client)
	confirmMsg, _ := json.Marshal(ResponseMessage{Content: "You are now in the matchmaking queue. Waiting for an opponent..."})
	sendActionMessage(client.Encoder, "queue_success", confirmMsg)
	fmt.Printf("Client %s added to the queue. Queue now has %d client(s).\n", client.IP, len(waitingQueueManager.WaitingQueue))
	tryMatch()
}

// handleOpenPack lida com a solicitação do cliente para abrir um pacote de cartas.
func handleOpenPack(client *Client) {
	globalStock.mu.Lock()
	defer globalStock.mu.Unlock()

	fmt.Printf("Client %s is opening a card pack...\n", client.IP)

	availableCards := make([]game.Card, 0)
	for card, quantity := range globalStock.Cards {
		for i := 0; i < quantity; i++ {
			availableCards = append(availableCards, card)
		}
	}

	if len(availableCards) < 3 {
		fmt.Println("Not enough cards in the global stock to open a pack.")
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

	fmt.Printf("Client %s received: %v. New stock: %v\n", client.IP, receivedCards, client.GetStock())

	payload := OpenPackPayload{
		Content:  "You opened a pack and received 3 new cards!",
		NewCards: receivedCards,
		NewStock: client.GetStock(),
	}
	responseData, _ := json.Marshal(payload)
	sendActionMessage(client.Encoder, "open_pack_success", responseData)
}

func tryMatch() {
	// Bloqueia o acesso à fila durante toda a operação de pareamento.
	waitingQueueManager.locker.Lock()
	defer waitingQueueManager.locker.Unlock()

	// Continua a formar pares enquanto houver clientes suficientes na fila.
	// Este loop garante que todos os pares possíveis sejam formados em uma única operação atômica.
	for len(waitingQueueManager.WaitingQueue) >= 2 {
		// Pega os dois primeiros clientes da fila
		c1 := waitingQueueManager.WaitingQueue[0]
		c2 := waitingQueueManager.WaitingQueue[1]
		
		// Remove-os da fila
		waitingQueueManager.WaitingQueue = waitingQueueManager.WaitingQueue[2:]

		// A lógica de pareamento e início do jogo agora está DENTRO do mesmo lock.
		fmt.Printf("Clients paired: %s vs %s\n", c1.IP, c2.IP)
		pairedClientsManager.MatchAdder(c1, c2)

		pairedMsg, _ := json.Marshal(ResponseMessage{Content: "You have been paired! The game is starting..."})
		sendActionMessage(c1.Encoder, "matched", pairedMsg)
		sendActionMessage(c2.Encoder, "matched", pairedMsg)

		go startGame(c1, c2)
	}
}

func handlePing(client *Client) {
	start := time.Now()
	pongMsg, _ := json.Marshal(ResponseMessage{Content: "PONG"})
	sendActionMessage(client.Encoder, "pong", pongMsg)
	latency := time.Since(start).Milliseconds()
	fmt.Printf("Ping from %s: %dms\n", client.IP, latency)
	latencyMsg, _ := json.Marshal(ResponseMessage{Content: fmt.Sprintf("Ping: %dms", latency)})
	sendActionMessage(client.Encoder, "pong_response", latencyMsg)
}

func handlePlay(client *Client, msg Message) {
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

func startGame(c1, c2 *Client) {
	fmt.Printf("Starting match between %s and %s\n", c1.IP, c2.IP)
	for {
		promptMsg, _ := json.Marshal(ResponseMessage{Content: "Your turn to play!"})
		sendActionMessage(c1.Encoder, "game_prompt_play", promptMsg)
		sendActionMessage(c2.Encoder, "game_prompt_play", promptMsg)

		move1, ok1 := <-c1.MoveChan
		move2, ok2 := <-c2.MoveChan

		if !ok1 || !ok2 {
			fmt.Printf("A player disconnected. Ending match between %s and %s.\n", c1.IP, c2.IP)
			if ok1 {
				gameOverMsg, _ := json.Marshal(GameOverPayload{Content: "Your opponent disconnected. You win!", FinalStock: c1.GetStock()})
				sendActionMessage(c1.Encoder, "game_over", gameOverMsg)
			}
			if ok2 {
				gameOverMsg, _ := json.Marshal(GameOverPayload{Content: "Your opponent disconnected. You win!", FinalStock: c2.GetStock()})
				sendActionMessage(c2.Encoder, "game_over", gameOverMsg)
			}
			return
		}

		c1.SetPlayedCard(move1)
		c2.SetPlayedCard(move2)
		c1.RemoveCard(move1)
		c2.RemoveCard(move2)

		winner, loser := game.DetermineWinner(c1, c2)

		var result string
		if winner == nil {
			result = fmt.Sprintf("Draw! Both played %s.", c1.GetPlayedCard())
		} else {
			result = fmt.Sprintf("Round winner: %s with %s. Loser: %s with %s.", winner.GetName(), winner.GetPlayedCard(), loser.GetName(), loser.GetPlayedCard())
			game.SwapCards(winner, loser)
		}

		resultMsg, _ := json.Marshal(ResponseMessage{Content: result})
		sendActionMessage(c1.Encoder, "game_round_result", resultMsg)
		sendActionMessage(c2.Encoder, "game_round_result", resultMsg)

		if len(c1.GetStock()) == 0 || len(c2.GetStock()) == 0 {
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

			fmt.Printf("Match finished between %s and %s. Winner: %s\n", c1.IP, c2.IP, finalWinner.IP)
			return
		}
	}
}

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