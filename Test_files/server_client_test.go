package server_test

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// Message representa a estrutura de uma mensagem, agora com um campo "Action"
// para roteamento. Esta é a versão do lado do cliente para os testes.
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// PairedMessage é um tipo de mensagem para notificar o cliente sobre o pareamento.
type PairedMessage struct {
	Content string `json:"content"`
}

// isServerRunning verifica se o servidor já está rodando.
func isServerRunning() bool {
	conn, err := net.DialTimeout("tcp", "localhost:8080", 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// connectAndTest é uma função auxiliar para lidar com a lógica de conexão, ping e pareamento.
func connectAndTest(t *testing.T, id int) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Errorf("Cliente %d falhou ao conectar: %v", id, err)
		return
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Espera pela mensagem de pareamento
	var matchedMsg Message
	err = decoder.Decode(&matchedMsg)
	if err != nil {
		t.Errorf("Cliente %d não recebeu a mensagem de pareamento: %v", id, err)
		return
	}

	if matchedMsg.Action != "matched" {
		t.Errorf("Cliente %d esperava 'matched', mas recebeu '%s'", id, matchedMsg.Action)
	}

	// Envia uma mensagem de ping para o servidor
	pingData, _ := json.Marshal(map[string]string{"message": "PING"})
	pingMsg := Message{
		Action: "ping",
		Data:   json.RawMessage(pingData),
	}

	err = encoder.Encode(pingMsg)
	if err != nil {
		t.Errorf("Cliente %d falhou ao enviar ping: %v", id, err)
		return
	}

	// Espera a resposta de pong do servidor
	var pongMsg Message
	err = decoder.Decode(&pongMsg)
	if err != nil {
		t.Errorf("Cliente %d não recebeu a resposta de pong: %v", id, err)
		return
	}

	if pongMsg.Action != "pong_response" {
		t.Errorf("Cliente %d esperava 'pong_response', mas recebeu '%s'", id, pongMsg.Action)
	}

	fmt.Printf("Cliente %d testado com sucesso.\n", id)
}

// TestSingleClientCommunication testa a conexão e o fluxo de ping-pong com um único cliente.
// Note: Este teste irá falhar, pois o servidor espera dois clientes para emparelhar.
// Ele é mantido aqui para mostrar o fluxo, mas o TestMultipleClientsCommunication é mais realista.
func TestSingleClientCommunication(t *testing.T) {
	if !isServerRunning() {
		t.Skip("Servidor não está em execução. Inicie manualmente antes de rodar os testes.")
	}

	t.Log("Este teste irá falhar, pois o servidor precisa de dois clientes para emparelhar.")
	t.Log("Execute o TestMultipleClientsCommunication para um teste mais realista.")

	connectAndTest(t, 1)
}

// TestMultipleClientsCommunication testa a comunicação e o pareamento com múltiplos clientes.
func TestMultipleClientsCommunication(t *testing.T) {
	if !isServerRunning() {
		t.Skip("Servidor não está em execução. Inicie manualmente antes de rodar os testes.")
	}

	var wg sync.WaitGroup
	numClients := 2 // Altere para um número par para que todos os clientes sejam pareados

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			connectAndTest(t, id)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Todos os clientes completaram o teste com sucesso.")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout: clientes demoraram demais para responder.")
	}
}
