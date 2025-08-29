package Test_files

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"PBL1_Redes_Chat_de_texto/server"
)

// connectClient conecta um cliente e lê mensagens recebidas
func connectClient(t *testing.T, username string, readTimeout time.Duration) (net.Conn, []server.Message) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Erro ao conectar cliente %s: %v", username, err)
	}

	// Envia registro inicial
	initialMessage := server.Message{Sender: username}
	initialData, _ := json.Marshal(initialMessage)
	conn.Write(initialData)

	var messages []server.Message
	buffer := make([]byte, 1024)
	deadline := time.Now().Add(readTimeout)

	for time.Now().Before(deadline) {
		conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, err := conn.Read(buffer)
		if err != nil {
			continue
		}
		var msg server.Message
		if err := json.Unmarshal(buffer[:n], &msg); err == nil {
			messages = append(messages, msg)
		}
	}

	return conn, messages
}

func TestClientRequeueOnDisconnect(t *testing.T) {
	// Inicia o servidor
	go server.Start()
	time.Sleep(300 * time.Millisecond) // espera servidor iniciar

	// Conecta dois clientes
	conn1, messages1 := connectClient(t, "User1", 500*time.Millisecond)
	defer conn1.Close()

	conn2, messages2 := connectClient(t, "User2", 500*time.Millisecond)
	defer conn2.Close()

	// Verifica se ambos receberam a mensagem de pareamento
	paired1 := false
	paired2 := false
	for _, msg := range messages1 {
		if len(msg.Content) >= 24 && msg.Content[:24] == "Você foi conectado com" {
			paired1 = true
			break
		}
	}
	for _, msg := range messages2 {
		if len(msg.Content) >= 24 && msg.Content[:24] == "Você foi conectado com" {
			paired2 = true
			break
		}
	}

	if !paired1 || !paired2 {
		t.Fatalf("Os clientes não foram pareados corretamente")
	}

	// Desconecta o segundo cliente
	conn2.Close()
	time.Sleep(200 * time.Millisecond) // espera o servidor processar a desconexão

	// Conecta um novo cliente para testar re-pareamento
	conn3, messages3 := connectClient(t, "User3", 500*time.Millisecond)
	defer conn3.Close()

	// Verifica se o primeiro cliente foi colocado de volta na fila de espera
	returnedToQueue := false
	for _, msg := range messages1 {
		if msg.Content == "Você voltou à fila de espera para novo pareamento..." {
			returnedToQueue = true
			break
		}
	}

	if !returnedToQueue {
		t.Errorf("O cliente que permaneceu não foi re-enviado à fila de espera após desconexão do parceiro")
	}

	// Verifica se um novo par foi formado
	newPairFormed := false
	for _, msg := range messages1 {
		if len(msg.Content) >= 24 && msg.Content[:24] == "Você foi conectado com" {
			newPairFormed = true
			break
		}
	}
	for _, msg := range messages3 {
		if len(msg.Content) >= 24 && msg.Content[:24] == "Você foi conectado com" {
			newPairFormed = true
			break
		}
	}

	if !newPairFormed {
		t.Errorf("Não foi formado novo pareamento após reconexão")
	}
}
