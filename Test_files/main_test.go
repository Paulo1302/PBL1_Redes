// File: server/main_test.go
package server_test // CORREÇÃO: Alterado de 'game' para 'server_test'

import (
	"PBL1_Redes-card_game/server"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

// --- Gerenciamento do Ciclo de Vida dos Testes ---

// TestMain é executado uma vez para todo o pacote. Inicia o servidor antes
// de todos os testes e o desliga no final, criando um ambiente estável.
func TestMain(m *testing.M) {
	fmt.Println("Setting up test suite, starting server...")
	shutdown := server.Start()
	// Aguarda um instante para garantir que o listener do servidor esteja pronto.
	time.Sleep(100 * time.Millisecond)

	// Roda todos os testes do pacote
	exitCode := m.Run()

	fmt.Println("Tearing down test suite, shutting down server...")
	shutdown()
	os.Exit(exitCode)
}

// --- Estruturas e Funções Auxiliares Comuns ---

type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type RegisterPayload struct {
	Name  string            `json:"name"`
	Stock map[string]int `json:"stock"`
}

// connectAndRegister é uma função auxiliar para conectar e registrar um cliente de teste.
// Aceita um estoque inicial para permitir testes de cenários variados.
func connectAndRegister(t *testing.T, name string, initialStock map[string]int) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", "server:8080") // Usa o nome do serviço Docker
	if err != nil {
		t.Fatalf("Client %s: Failed to connect to server: %v", name, err)
	}

	initialData := RegisterPayload{Name: name, Stock: initialStock}
	registerData, _ := json.Marshal(initialData)
	registerMsg := Message{Action: "register", Data: registerData}

	if err := json.NewEncoder(conn).Encode(registerMsg); err != nil {
		conn.Close()
		t.Fatalf("Client %s: Failed to send register message: %v", name, err)
	}
	return conn
}

// readMessage lê a próxima mensagem da conexão, tratando timeouts.
func readMessage(t *testing.T, conn net.Conn, timeout time.Duration) (Message, error) {
	t.Helper()
	var msg Message
	conn.SetReadDeadline(time.Now().Add(timeout))
	err := json.NewDecoder(conn).Decode(&msg)
	return msg, err
}