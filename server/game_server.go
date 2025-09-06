package server

import(
	"net"
	"encoding/json"
	"fmt"
	"sync"
	"PBL1_Redes/game"
	"time"
)


type Client struct {
    Name        string
    Connection  net.Conn
    IP          string
    Stock     map[game.Carta]int
    PlayedCard game.Carta
}

type Message struct {
	Sender   string `json:"user"`
	Receiver string `json:"to"`
	Content  string `json:"content"`
}

type WaitingQueueManager struct{
	locker            sync.RWMutex 
	WaitingQueue []*Client
}

type PairedClientsManager struct {
	locker            sync.RWMutex 
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
	// Bloqueia o mutex de escrita para garantir acesso exclusivo
	m.locker.Lock()
	defer m.locker.Unlock()

	// Verifica se o cliente existe no mapa de pares
	if pairID, ok := m.PairedClients[client.IP]; ok {
		// Remove ambos os clientes do mapa de pares
		delete(m.PairedClients, client.IP)
		delete(m.PairedClients, pairID)

		fmt.Printf("Pares de clientes removidos com sucesso: %s e %s.\n", client.IP, pairID)

	} else {
		// Se o cliente não estiver no mapa, nada a fazer
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
	fmt.Println("Servidor de chat iniciado na porta 8080")
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
	}

	fmt.Printf("Novo cliente conectado: %s\n", client.IP)

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// adiciona cliente na fila de espera
	waitingQueueManager.WQueueAdder(client)
	fmt.Printf("Cliente %s adicionado à fila de espera\n", client.IP)

	// tenta emparelhar com outro cliente
	var opponent *Client
	waitingQueueManager.locker.Lock()
	if len(waitingQueueManager.WaitingQueue) >= 2 {
		if waitingQueueManager.WaitingQueue[0].IP == client.IP {
			opponent = waitingQueueManager.WaitingQueue[1]
		} else {
			opponent = waitingQueueManager.WaitingQueue[0]
		}
		// remove ambos da fila
		waitingQueueManager.WQueueRemover(client)
		waitingQueueManager.WQueueRemover(opponent)
		pairedClientsManager.MatchAdder(client, opponent)

		fmt.Printf("Clientes emparelhados: %s vs %s\n", client.IP, opponent.IP)

		// notifica ambos
		json.NewEncoder(client.Connection).Encode(Message{
			Sender:   "Servidor",
			Receiver: client.Name,
			Content:  "Você foi pareado! Jogo começando...",
		})
		json.NewEncoder(opponent.Connection).Encode(Message{
			Sender:   "Servidor",
			Receiver: opponent.Name,
			Content:  "Você foi pareado! Jogo começando...",
		})

		go startGame(client, opponent)
	}
	waitingQueueManager.locker.Unlock()

	// loop para manter conexão e ouvir mensagens (ex: sair do jogo)
	for {
		var msg Message
		err := decoder.Decode(&msg)
		if err != nil {
			fmt.Printf("Cliente %s desconectado: %v\n", client.IP, err)
			waitingQueueManager.WQueueRemover(client)
			pairedClientsManager.MatchRemover(client)
			return
		}
		fmt.Printf("Mensagem de %s: %s\n", msg.Sender, msg.Content)
		// apenas ecoa por enquanto
		encoder.Encode(Message{
			Sender:   "Servidor",
			Receiver: msg.Sender,
			Content:  "Mensagem recebida!",
		})
	}
}

// Função que executa rodadas entre dois clientes pareados
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

		json.NewEncoder(c1.Connection).Encode(Message{
			Sender:   "Servidor",
			Receiver: c1.Name,
			Content:  resultado,
		})
		json.NewEncoder(c2.Connection).Encode(Message{
			Sender:   "Servidor",
			Receiver: c2.Name,
			Content:  resultado,
		})

		time.Sleep(3 * time.Second) // pausa entre rodadas
	}
}

