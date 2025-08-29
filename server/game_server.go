package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
)

// Message representa uma mensagem entre clientes
type Message struct {
	Sender   string `json:"user"`
	Receiver string `json:"to"`
	Content  string `json:"content"`
}

// Client representa um usuário conectado
type Client struct {
	Name       string
	Connection net.Conn
	IP         string
}

// MatchManager gerencia clientes ativos, fila de espera e pareamentos
type MatchManager struct {
	ActiveClients map[string]*Client
	WaitingQueue  []*Client
	PairedClients map[string]string
	Mutex         sync.Mutex
	NewClientCh   chan struct{}
}

// Cria um novo MatchManager
func NewMatchManager() *MatchManager {
	return &MatchManager{
		ActiveClients: make(map[string]*Client),
		WaitingQueue:  []*Client{},
		PairedClients: make(map[string]string),
		NewClientCh:   make(chan struct{}, 1),
	}
}

// Adiciona cliente ao gerenciamento
func (m *MatchManager) AddClient(client *Client) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	normalized := strings.ToLower(client.Name)
	m.ActiveClients[normalized] = client
	m.WaitingQueue = append(m.WaitingQueue, client)

	// Sinaliza para formar pares
	select {
	case m.NewClientCh <- struct{}{}:
	default:
	}
}

// Remove cliente do gerenciamento
func (m *MatchManager) RemoveClient(clientName string) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	normalized := strings.ToLower(clientName)
	client, exists := m.ActiveClients[normalized]
	if !exists {
		return
	}

	delete(m.ActiveClients, normalized)

	// Remove da fila de espera
	for i, queued := range m.WaitingQueue {
		if strings.ToLower(queued.Name) == normalized {
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			break
		}
	}

	// Remove pareamento
	if partnerName, paired := m.PairedClients[normalized]; paired {
		delete(m.PairedClients, normalized)
		delete(m.PairedClients, partnerName)

		if partner, ok := m.ActiveClients[partnerName]; ok {
			notifyClient(partner, fmt.Sprintf("Seu parceiro %s desconectou e você voltou à fila de espera.", client.Name))
			m.WaitingQueue = append(m.WaitingQueue, partner)
			select {
			case m.NewClientCh <- struct{}{}:
			default:
			}
		}
	}

	client.Connection.Close()
}

// Loop contínuo para formar pares
func (m *MatchManager) RunPairing() {
	for {
		<-m.NewClientCh

		m.Mutex.Lock()
		for len(m.WaitingQueue) >= 2 {
			first := m.WaitingQueue[0]
			second := m.WaitingQueue[1]
			m.WaitingQueue = m.WaitingQueue[2:]

			fName := strings.ToLower(first.Name)
			sName := strings.ToLower(second.Name)

			m.PairedClients[fName] = sName
			m.PairedClients[sName] = fName

			notifyClient(first, "Você foi conectado com "+second.Name)
			notifyClient(second, "Você foi conectado com "+first.Name)

			fmt.Printf("Par formado: %s <-> %s\n", first.Name, second.Name)
		}
		m.Mutex.Unlock()
	}
}

// Retorna parceiro de um cliente
func (m *MatchManager) GetPartner(clientName string) (*Client, bool) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	normalized := strings.ToLower(clientName)
	partnerName, paired := m.PairedClients[normalized]
	if !paired {
		return nil, false
	}

	partner, exists := m.ActiveClients[partnerName]
	return partner, exists
}

// -------------------------------------------------------------------
var GlobalMatchManager = NewMatchManager()

// Start inicializa o servidor e aguarda conexões
func Start() {
	fmt.Println("Servidor de chat iniciado na porta 8080")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor:", err)
		return
	}
	defer listener.Close()

	go GlobalMatchManager.RunPairing()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		go handleClientConnection(conn)
	}
}

// -------------------------------------------------------------------
func handleClientConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)

	// Lê mensagem inicial (nome do usuário)
	data, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return
	}

	var initialMsg Message
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &initialMsg); err != nil || initialMsg.Sender == "" {
		conn.Write([]byte(`{"user":"Servidor","content":"Conexão inválida"}\n`))
		conn.Close()
		return
	}

	client := &Client{
		Name:       initialMsg.Sender,
		Connection: conn,
		IP:         conn.RemoteAddr().String(),
	}

	fmt.Printf("🔹 Usuário conectado: %s | IP: %s\n", client.Name, client.IP)
	GlobalMatchManager.AddClient(client)
	notifyClient(client, "Aguardando outro usuário se conectar...")

	for {
		data, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("❌ Usuário desconectado: %s | IP: %s\n", client.Name, client.IP)
			GlobalMatchManager.RemoveClient(client.Name)
			return
		}

		var msg Message
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &msg); err != nil {
			continue
		}

		partner, ok := GlobalMatchManager.GetPartner(msg.Sender)
		if ok && partner != nil {
			outMsg := Message{
				Sender:   msg.Sender,
				Receiver: partner.Name,
				Content:  msg.Content,
			}
			jsonData, err := json.Marshal(outMsg)
			if err == nil {
				_, _ = partner.Connection.Write(append(jsonData, '\n'))
				fmt.Printf("📩 %s -> %s | Mensagem: %s\n", msg.Sender, partner.Name, msg.Content)
			}
		} else {
			notifyClient(client, "Ainda não há parceiro conectado.")
		}
	}
}

// -------------------------------------------------------------------
func notifyClient(client *Client, content string) {
	msg := Message{
		Sender:   "Servidor",
		Receiver: client.Name,
		Content:  content,
	}
	data, err := json.Marshal(msg)
	if err == nil {
		_, _ = client.Connection.Write(append(data, '\n'))
	}
}
