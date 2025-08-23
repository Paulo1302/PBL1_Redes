package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// Estrutura da mensagem enviada entre clientes e servidor
type Message struct {
	User    string `json:"user"`    // remetente
	To      string `json:"to"`      // destinatário
	Content string `json:"content"` // conteúdo da mensagem
}

// Estrutura que representa um cliente conectado
type Client struct {
	Name       string
	Connection net.Conn
	IP         string
}

// Variáveis globais para gerenciar clientes e pares
var (
	waitingQueue   []Client               // fila de clientes aguardando pareamento
	pairedClients  = make(map[string]string) // mapa de pares (usuário -> parceiro)
	activeClients  = make(map[string]Client) // clientes atualmente conectados
	mutex          sync.Mutex             // mutex para sincronização entre goroutines
)

func main() {
	// Inicializa o servidor na porta 8080
	fmt.Println("Servidor de chat iniciado na porta 8080")
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Erro ao iniciar servidor:", err)
		return
	}
	defer listener.Close()

	// Loop principal: aceita conexões de clientes
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Erro ao aceitar conexão:", err)
			continue
		}
		// Cada conexão é tratada em uma goroutine separada
		go handleClientConnection(conn)
	}
}

// Função que trata cada cliente conectado
func handleClientConnection(conn net.Conn) {
	defer conn.Close() // garante fechamento da conexão ao sair

	var buffer bytes.Buffer
	readBuffer := make([]byte, 1024)

	// Recebe a primeira mensagem do cliente (registro com o nome)
	n, err := conn.Read(readBuffer)
	if err != nil {
		return
	}
	buffer.Write(readBuffer[:n])

	// Desserializa a mensagem inicial
	var initialMessage Message
	if err := json.Unmarshal(buffer.Bytes(), &initialMessage); err != nil || initialMessage.User == "" {
		conn.Write([]byte(`{"user":"Servidor","content":"Conexão inválida"}`))
		return
	}

	// Cria a estrutura Client com informações do usuário
	clientIP := conn.RemoteAddr().String()
	client := Client{
		Name:       initialMessage.User,
		Connection: conn,
		IP:         clientIP,
	}

	fmt.Printf("🔹 Usuário conectado: %s | IP: %s\n", client.Name, client.IP)

	// Adiciona o cliente aos mapas globais e tenta pareamento
	mutex.Lock()
	activeClients[client.Name] = client
	waitingQueue = append(waitingQueue, client)

	if len(waitingQueue) < 2 {
		// Se não houver outro usuário, informa que está aguardando
		notifyClient(client, "Aguardando outro usuário se conectar...")
		mutex.Unlock()
	} else {
		// Se houver pelo menos dois usuários, forma um par
		firstClient := waitingQueue[0]
		secondClient := waitingQueue[1]
		waitingQueue = waitingQueue[2:]

		pairedClients[firstClient.Name] = secondClient.Name
		pairedClients[secondClient.Name] = firstClient.Name

		fmt.Printf("Par formado: %s <-> %s\n", firstClient.Name, secondClient.Name)

		// Notifica os dois clientes que foram pareados
		notifyClient(firstClient, "Você foi conectado com "+secondClient.Name)
		notifyClient(secondClient, "Você foi conectado com "+firstClient.Name)
		mutex.Unlock()
	}

	buffer.Reset()

	// Loop para receber mensagens do cliente
	for {
		n, err := conn.Read(readBuffer)
		if err != nil {
			// Cliente desconectou
			fmt.Printf("❌ Usuário desconectado: %s | IP: %s\n", client.Name, client.IP)

			mutex.Lock()
			delete(activeClients, client.Name) // remove cliente ativo

			// Notifica o parceiro sobre desconexão e remove par
			if partnerName, ok := pairedClients[client.Name]; ok {
				if partner, exists := activeClients[partnerName]; exists {
					notifyClient(partner, fmt.Sprintf("Seu parceiro %s desconectou.", client.Name))
				}
				delete(pairedClients, partnerName)
				delete(pairedClients, client.Name)
			}

			// Remove cliente da fila de espera, se estava nela
			for i, queuedClient := range waitingQueue {
				if queuedClient.Name == client.Name {
					waitingQueue = append(waitingQueue[:i], waitingQueue[i+1:]...)
					break
				}
			}

			mutex.Unlock()
			return
		}

		// Acumula os dados recebidos
		buffer.Write(readBuffer[:n])
		var receivedMessage Message
		if err := json.Unmarshal(buffer.Bytes(), &receivedMessage); err != nil {
			buffer.Reset()
			continue
		}
		buffer.Reset()

		// Envia a mensagem para o parceiro, se houver
		mutex.Lock()
		partnerName, paired := pairedClients[receivedMessage.User]
		partnerClient, exists := activeClients[partnerName]
		mutex.Unlock()

		if paired && exists {
			// Cria resposta para o parceiro
			response := Message{
				User:    receivedMessage.User,
				To:      partnerName,
				Content: receivedMessage.Content,
			}
			data, _ := json.Marshal(response)
			partnerClient.Connection.Write(data)

			fmt.Printf("📩 %s -> %s | IP: %s -> %s | Mensagem: %s\n",
				receivedMessage.User, partnerName, client.IP, partnerClient.IP, receivedMessage.Content)
		} else {
			// Informa ao cliente que ainda não há parceiro conectado
			notifyClient(client, "Ainda não há parceiro conectado.")
		}
	}
}

// Função para enviar mensagens de status do servidor a um cliente
func notifyClient(client Client, message string) {
	response := Message{
		User:    "Servidor",
		To:      client.Name,
		Content: message,
	}
	data, _ := json.Marshal(response)
	client.Connection.Write(data)
}
