package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"PBL1_Redes/game"
)

// Client representa um cliente conectado, com todas as ferramentas de comunicação e dados de jogo.
type Client struct {
	Name        string
	Connection  net.Conn
	IP          string
	Stock       map[game.Carta]int
	PlayedCard  game.Carta
	Reader      *bufio.Reader
	Writer      *bufio.Writer
	Encoder     *json.Encoder
	Decoder     *json.Decoder
}

// Message representa a estrutura de uma mensagem, agora com um campo "Action"
// para roteamento.
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// PairedMessage é um tipo de mensagem para notificar o cliente sobre o pareamento.
type PairedMessage struct {
	Content string `json:"content"`
}

type WaitingQueueManager struct {
	locker       sync.RWMutex
	WaitingQueue []*Client
}

type PairedClientsManager struct {
	locker        sync.RWMutex
	PairedClients map[string]string
}

// Implementações da interface game.Jogador para Client
func (c *Client) GetNome() string {
	return c.Name
}

func (c *Client) GetEstoque() map[game.Carta]int {
	return c.Stock
}

func (c *Client) GetCartaJogada() game.Carta {
	return c.PlayedCard
}

func (c *Client) SetCartaJogada(card game.Carta) {
	c.PlayedCard = card
}

func (c *Client) RemoverCarta(card game.Carta) {
	if c.Stock[card] > 0 {
		c.Stock[card]--
		if c.Stock[card] == 0 {
			delete(c.Stock, card)
		}
	}
}

func (c *Client) AdicionarCarta(card game.Carta) {
	c.Stock[card]++
}

func (m *PairedClientsManager) MatchAdder(first_client *Client, second_client *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()

	m.PairedClients[first_client.IP] = second_client.IP
	m.PairedClients[second_client.IP] = first_client.IP

	fmt.Println("Pares criados com sucesso.")
}

func (m *PairedClientsManager) MatchRemover(client *Client) {
	m.locker.Lock()
	defer m.locker.Unlock()

	if pairID, ok := m.PairedClients[client.IP]; ok {
		delete(m.PairedClients, client.IP)
		delete(m.PairedClients, pairID)
		fmt.Printf("Pares de clientes removidos com sucesso: %s e %s.\n", client.IP, pairID)
	} else {
		fmt.Printf("Cliente %s não encontrado no mapa de pares.\n", client.IP)
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

func Start() {
	fmt.Println("Servidor de jogo iniciado na porta 8080")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor:", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		go handleClientConnection(conn)
	}
}

var (
	waitingQueueManager  = &WaitingQueueManager{WaitingQueue: []*Client{}}
	pairedClientsManager = &PairedClientsManager{PairedClients: make(map[string]string)}
)

// handleClientConnection agora usa um loop de decodificação e um switch para rotear as ações.
func handleClientConnection(conn net.Conn) {
	defer conn.Close()

	client := &Client{
		Name:       conn.RemoteAddr().String(),
		Connection: conn,
		IP:         conn.RemoteAddr().String(),
		Stock: map[game.Carta]int{
			game.Pedra:   3,
			game.Papel:   3,
			game.Tesoura: 3,
		},
		Encoder: json.NewEncoder(conn),
		Decoder: json.NewDecoder(conn),
	}

	fmt.Printf("Novo cliente conectado: %s\n", client.IP)

	// Ações iniciais ao conectar

	fmt.Printf("Cliente %s adicionado à fila de espera\n", client.IP)
	tryMatch(client)

	for {
		var msg Message
		err := client.Decoder.Decode(&msg)
		if err != nil {
			fmt.Printf("Cliente %s desconectado: %v\n", client.IP, err)
			waitingQueueManager.WQueueRemover(client)
			pairedClientsManager.MatchRemover(client)
			return
		}

		// Roteamento de ações
		switch msg.Action {
		case "ping":
			handlePing(client, msg)
		case "jogar":
			handlePlay(client, msg)
		default:
			sendErrorMessage(client.Encoder, "Ação desconhecida.")
		}
	}
}

// tryMatch tenta emparelhar um cliente assim que ele entra na fila de espera.
func tryMatch(client *Client) {
	waitingQueueManager.locker.Lock()
	defer waitingQueueManager.locker.Unlock()

	if len(waitingQueueManager.WaitingQueue) >= 2 {
		var opponent *Client
		if waitingQueueManager.WaitingQueue[0].IP == client.IP {
			opponent = waitingQueueManager.WaitingQueue[1]
		} else {
			opponent = waitingQueueManager.WaitingQueue[0]
		}

		waitingQueueManager.WQueueRemover(client)
		waitingQueueManager.WQueueRemover(opponent)
		pairedClientsManager.MatchAdder(client, opponent)

		fmt.Printf("Clientes emparelhados: %s vs %s\n", client.IP, opponent.IP)

		// Envia mensagens de pareamento para ambos os clientes
		pairedMsg, _ := json.Marshal(PairedMessage{Content: "Você foi pareado! Jogo começando..."})
		sendActionMessage(client.Encoder, "matched", pairedMsg)
		sendActionMessage(opponent.Encoder, "matched", pairedMsg)

		go startGame(client, opponent)
	}
}

// handlePing responde a uma ação de ping.
func handlePing(client *Client, msg Message) {
	start := time.Now()
	// Envia a mensagem de resposta "PONG" para o cliente
	pongMsg, _ := json.Marshal(PairedMessage{Content: "PONG"})
	sendActionMessage(client.Encoder, "pong", pongMsg)

	// Calcula a latência e a converte para milissegundos
	latency := time.Since(start).Milliseconds()

	fmt.Printf("Ping de %s: %dms\n", client.IP, latency)

	// Envia a latência de volta para o cliente
	latencyMsg, _ := json.Marshal(PairedMessage{Content: fmt.Sprintf("Ping: %dms", latency)})
	sendActionMessage(client.Encoder, "pong_response", latencyMsg)
}

// handlePlay lida com a ação de um jogador jogar uma carta.
func handlePlay(client *Client, msg Message) {
	// Lógica para lidar com a carta jogada pelo cliente.
	// Por exemplo, decodificar o payload da carta.
	// Nota: A lógica do jogo em si (startGame) é independente deste handler,
	// mas este é o ponto de entrada para o cliente enviar a jogada.
	//
	// Exemplo de payload: {"carta_escolhida": "Pedra"}
	// var payload struct {
	//    CartaEscolhida game.Carta `json:"carta_escolhida"`
	// }
	//
	// json.Unmarshal(msg.Data, &payload)
	// client.SetCartaJogada(payload.CartaEscolhida)
	//
	fmt.Printf("Recebi ação 'jogar' do cliente %s\n", client.IP)
	sendActionMessage(client.Encoder, "game_status", json.RawMessage(`{"status": "Jogada recebida!"}`))
}

// startGame é uma goroutine que gerencia o fluxo de uma partida.
func startGame(c1, c2 *Client) {
	for {
		vencedor, perdedor := game.JogaRodada(c1, c2)

		var resultado string
		if vencedor == nil {
			resultado = fmt.Sprintf("Empate! Ambos jogaram %s", c1.PlayedCard)
		} else {
			resultado = fmt.Sprintf("Vencedor: %s com %s", vencedor.GetNome(), vencedor.GetCartaJogada())
			game.TrocarCartas(vencedor, perdedor)
		}

		resultMsg, _ := json.Marshal(PairedMessage{Content: resultado})
		sendActionMessage(c1.Encoder, "game_round_result", resultMsg)
		sendActionMessage(c2.Encoder, "game_round_result", resultMsg)

		time.Sleep(3 * time.Second)
	}
}

// sendActionMessage é uma função auxiliar para enviar mensagens no formato de Ação.
func sendActionMessage(encoder *json.Encoder, action string, data json.RawMessage) {
	finalMsg := Message{
		Action: action,
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}

// sendErrorMessage é uma função auxiliar para enviar mensagens de erro.
func sendErrorMessage(encoder *json.Encoder, errorMsg string) {
	payload := map[string]string{"error": errorMsg}
	data, _ := json.Marshal(payload)
	finalMsg := Message{
		Action: "error",
		Data:   data,
	}
	_ = encoder.Encode(finalMsg)
}