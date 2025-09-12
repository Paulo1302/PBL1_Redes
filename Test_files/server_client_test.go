// File: Test_files/game_server_test.go
package server_test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"PBL1_Redes-card_game/server"
	"sync"
	"testing"
	"time"
)

// --- Gerenciamento do Ciclo de Vida dos Testes ---

// TestMain é executado uma vez para todo o pacote. Inicia o servidor antes
// de todos os testes e o desliga no final, criando um ambiente estável.
func TestMain(m *testing.M) {
	fmt.Println("Setting up test suite, starting server...")
	shutdown := server.Start()
	time.Sleep(100 * time.Millisecond) // Dá tempo para o servidor iniciar

	// Roda todos os testes
	exitCode := m.Run()

	fmt.Println("Tearing down test suite, shutting down server...")
	shutdown()
	os.Exit(exitCode)
}

// --- Estruturas e Funções Auxiliares ---

type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}
type RegisterPayload struct {
	Name  string         `json:"name"`
	Stock map[string]int `json:"stock"`
}

// connectAndRegister é uma função auxiliar para conectar e registrar um cliente de teste.
func connectAndRegister(t *testing.T, name string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Fatalf("Failed to connect for client %s: %v", name, err)
	}

	initialData := RegisterPayload{
		Name:  name,
		Stock: map[string]int{"Rock": 1, "Paper": 1, "Scissors": 1},
	}
	registerData, _ := json.Marshal(initialData)
	registerMsg := Message{Action: "register", Data: registerData}

	if err := json.NewEncoder(conn).Encode(registerMsg); err != nil {
		t.Fatalf("Failed to send register message for client %s: %v", name, err)
	}
	return conn
}

// --- Casos de Teste ---

// TestFullGameFlow valida o fluxo de um jogo normal com dois jogadores.
func TestFullGameFlow(t *testing.T) {
	client1Conn := connectAndRegister(t, "Player1")
	defer client1Conn.Close()
	client2Conn := connectAndRegister(t, "Player2")
	defer client2Conn.Close()

	// Ambos os clientes entram na fila de pareamento
	queueMsg := Message{Action: "queue_for_match"}
	_ = json.NewEncoder(client1Conn).Encode(queueMsg)
	_ = json.NewEncoder(client2Conn).Encode(queueMsg)

	// Ambos devem receber a confirmação de entrada na fila e, em seguida, a de pareamento.
	decoder1 := json.NewDecoder(client1Conn)
	decoder2 := json.NewDecoder(client2Conn)
	var msg1, msg2 Message

	_ = decoder1.Decode(&msg1) // queue_success
	_ = decoder2.Decode(&msg2) // queue_success
	if msg1.Action != "queue_success" || msg2.Action != "queue_success" {
		t.Fatal("Did not receive queue_success from both clients")
	}

	_ = decoder1.Decode(&msg1) // matched
	_ = decoder2.Decode(&msg2) // matched
	if msg1.Action != "matched" {
		t.Errorf("Client 1 was not matched. Got: %s", msg1.Action)
	}
	if msg2.Action != "matched" {
		t.Errorf("Client 2 was not matched. Got: %s", msg2.Action)
	}
}

// TestStressMatchmaking valida a sincronização do pareamento sob carga.
func TestStressMatchmaking(t *testing.T) {
	numClients := 100 // Deve ser um número par para que todos possam ser pareados
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			defer wg.Done()
			clientName := fmt.Sprintf("StressClient_%d", id)

			// t.Run agrupa a lógica e o output para cada cliente
			t.Run(clientName, func(t *testing.T) {
				t.Parallel() // Permite que os sub-testes de cada cliente rodem em paralelo

				conn := connectAndRegister(t, clientName)
				defer conn.Close()

				// Envia a requisição para entrar na fila
				queueMsg := Message{Action: "queue_for_match"}
				if err := json.NewEncoder(conn).Encode(queueMsg); err != nil {
					t.Errorf("Failed to send queue message: %v", err)
					return
				}

				decoder := json.NewDecoder(conn)
				var response Message
				
				// Define um timeout para as respostas do servidor
				conn.SetReadDeadline(time.Now().Add(10 * time.Second))

				// Lê a confirmação de entrada na fila
				if err := decoder.Decode(&response); err != nil || response.Action != "queue_success" {
					t.Errorf("Did not receive queue_success in time. Action: %s, err: %v", response.Action, err)
					return
				}
				
				// Espera pela mensagem de pareamento
				if err := decoder.Decode(&response); err != nil || response.Action != "matched" {
					t.Errorf("Failed to get matched in time. Action: %s, err: %v", response.Action, err)
					return
				}
			})
		}(i)
	}
	
	// WaitGroup espera que todas as goroutines (e seus sub-testes) terminem.
	wg.Wait()
}
// File: Test_files/game_server_test.go

// TestStressPing aplica uma carga de múltiplos clientes enviando pings repetidamente
// para medir a capacidade de resposta do servidor a requisições de baixo custo.
func TestStressPing(t *testing.T) {
	concurrency := 100 // Número de clientes simultâneos
	pingsPerClient := 10 // Quantidade de pings que cada cliente enviará

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			clientName := fmt.Sprintf("PingClient_%d", id)

			t.Run(clientName, func(t *testing.T) {
				t.Parallel() // Permite que os clientes rodem em paralelo

				conn := connectAndRegister(t, clientName)
				defer conn.Close()

				encoder := json.NewEncoder(conn)
				decoder := json.NewDecoder(conn)
				pingMsg := Message{Action: "ping"}

				// Cada cliente envia múltiplos pings em sequência
				for j := 0; j < pingsPerClient; j++ {
					if err := encoder.Encode(pingMsg); err != nil {
						t.Errorf("Request #%d: Failed to send ping: %v", j+1, err)
						return // Interrompe o teste para este cliente em caso de erro
					}

					conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					var response Message
					if err := decoder.Decode(&response); err != nil {
						t.Errorf("Request #%d: Failed to receive pong: %v", j+1, err)
						return
					}

					if response.Action != "pong" {
						t.Errorf("Request #%d: Expected action 'pong' but got '%s'", j+1, response.Action)
						return
					}
				}
			})
		}(i)
	}

	// Espera que todos os clientes terminem de enviar todos os seus pings
	wg.Wait()
}