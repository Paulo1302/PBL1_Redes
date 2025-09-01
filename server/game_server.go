package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
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
	MsgQueue   chan Message
	Closed     chan struct{}
}

// MatchManager gerencia clientes ativos, fila de espera e pareamentos
type MatchManager struct {
	ActiveClients map[string]*Client
	WaitingQueue  []*Client
	PairedClients map[string]string

	ActiveMutex  sync.RWMutex
	QueueMutex   sync.Mutex
	PairedMutex  sync.RWMutex

	NewClientCh chan struct{}
}

func NewMatchManager() *MatchManager {
	return &MatchManager{
		ActiveClients: make(map[string]*Client),
		WaitingQueue:  []*Client{},
		PairedClients: make(map[string]string),
		NewClientCh:   make(chan struct{}, 1),
	}
}

// Adiciona cliente
func (m *MatchManager) AddClient(client *Client) {
	client.MsgQueue = make(chan Message, 50)
	client.Closed = make(chan struct{})

	m.ActiveMutex.Lock()
	m.ActiveClients[strings.ToLower(client.Name)] = client
	m.ActiveMutex.Unlock()

	m.QueueMutex.Lock()
	m.WaitingQueue = append(m.WaitingQueue, client)
	m.QueueMutex.Unlock()

	select {
	case m.NewClientCh <- struct{}{}:
	default:
	}

	go client.startWriter()
}

// Remove cliente
func (m *MatchManager) RemoveClient(clientName string) {
	n := strings.ToLower(clientName)

	m.ActiveMutex.Lock()
	client, exists := m.ActiveClients[n]
	if exists {
		delete(m.ActiveClients, n)
		close(client.Closed)
	}
	m.ActiveMutex.Unlock()

	if !exists {
		return
	}

	// Remove da fila de espera
	m.QueueMutex.Lock()
	for i, c := range m.WaitingQueue {
		if strings.ToLower(c.Name) == n {
			m.WaitingQueue = append(m.WaitingQueue[:i], m.WaitingQueue[i+1:]...)
			break
		}
	}
	m.QueueMutex.Unlock()

	// Remove pareamento
	m.PairedMutex.Lock()
	if partnerName, ok := m.PairedClients[n]; ok {
		delete(m.PairedClients, n)
		delete(m.PairedClients, partnerName)

		m.ActiveMutex.RLock()
		if partner, exists := m.ActiveClients[partnerName]; exists {
			notifyClient(partner, fmt.Sprintf("Seu parceiro %s desconectou e você voltou à fila de espera.", client.Name))
			m.QueueMutex.Lock()
			m.WaitingQueue = append(m.WaitingQueue, partner)
			m.QueueMutex.Unlock()
			select {
			case m.NewClientCh <- struct{}{}:
			default:
			}
		}
		m.ActiveMutex.RUnlock()
	}
	m.PairedMutex.Unlock()

	client.Connection.Close()
}

// Loop de pareamento
func (m *MatchManager) RunPairing() {
	for range m.NewClientCh {
		for {
			m.QueueMutex.Lock()
			if len(m.WaitingQueue) < 2 {
				m.QueueMutex.Unlock()
				break
			}
			first := m.WaitingQueue[0]
			second := m.WaitingQueue[1]
			m.WaitingQueue = m.WaitingQueue[2:]
			m.QueueMutex.Unlock()

			fName := strings.ToLower(first.Name)
			sName := strings.ToLower(second.Name)

			m.PairedMutex.Lock()
			m.PairedClients[fName] = sName
			m.PairedClients[sName] = fName
			m.PairedMutex.Unlock()

			notifyClient(first, "Você foi conectado com "+second.Name)
			notifyClient(second, "Você foi conectado com "+first.Name)

			fmt.Printf("Par formado: %s <-> %s\n", first.Name, second.Name)
		}
	}
}

// Retorna parceiro
func (m *MatchManager) GetPartner(clientName string) (*Client, bool) {
	n := strings.ToLower(clientName)
	m.PairedMutex.RLock()
	partnerName, paired := m.PairedClients[n]
	m.PairedMutex.RUnlock()
	if !paired {
		return nil, false
	}

	m.ActiveMutex.RLock()
	partner, exists := m.ActiveClients[partnerName]
	m.ActiveMutex.RUnlock()
	return partner, exists
}

// -------------------------------------------------------------------
var GlobalMatchManager = NewMatchManager()

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

	// Timeout para receber nome
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
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
	GlobalMatchManager.AddClient(client)
	notifyClient(client, "Aguardando outro usuário se conectar...")
	fmt.Printf("🔹 Usuário conectado: %s | IP: %s\n", client.Name, client.IP)

	for {
		// Timeout para leitura de cada mensagem
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		data, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
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
			partner.MsgQueue <- outMsg
			fmt.Printf("📩 %s -> %s | Mensagem: %s\n", msg.Sender, partner.Name, msg.Content)
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
	client.MsgQueue <- msg
}

// Cada cliente tem uma goroutine para enviar mensagens de forma assíncrona
func (c *Client) startWriter() {
	go func() {
		for {
			select {
			case msg := <-c.MsgQueue:
				data, err := json.Marshal(msg)
				if err == nil {
					c.Connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
					c.Connection.Write(append(data, '\n'))
				}
			case <-c.Closed:
				return
			}
		}
	}()
}
