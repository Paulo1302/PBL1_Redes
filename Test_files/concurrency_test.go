// File: server/concurrency_test.go
package server_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestStressMatchmaking valida a sincronização do pareamento sob carga.
func TestStressMatchmaking(t *testing.T) {
	// (Este teste foi mantido e aprimorado com o helper 'readMessage')
	numClients := 100 // Deve ser um número par
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			defer wg.Done()
			clientName := fmt.Sprintf("StressClient_%d", id)
			stock := map[string]int{"Rock": 1}

			t.Run(clientName, func(t *testing.T) {
				t.Parallel()
				conn := connectAndRegister(t, clientName, stock)
				defer conn.Close()

				queueMsg := Message{Action: "queue_for_match"}
				_ = json.NewEncoder(conn).Encode(queueMsg)

				// Lê a confirmação de entrada na fila e o pareamento
				if _, err := readMessage(t, conn, 15*time.Second); err != nil {
					t.Errorf("Did not receive queue_success in time: %v", err)
					return
				}
				if _, err := readMessage(t, conn, 15*time.Second); err != nil {
					t.Errorf("Failed to get matched in time: %v", err)
					return
				}
			})
		}(i)
	}
	wg.Wait()
}

// TestConcurrentPackOpening simula múltiplos clientes abrindo pacotes ao mesmo tempo
// para garantir que o estoque global do servidor seja manipulado de forma segura.
func TestConcurrentPackOpening(t *testing.T) {
	numClients := 100
	packsPerClient := 3
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			defer wg.Done()
			clientName := fmt.Sprintf("PackClient_%d", id)
			stock := make(map[string]int)

			t.Run(clientName, func(t *testing.T) {
				t.Parallel()
				conn := connectAndRegister(t, clientName, stock)
				defer conn.Close()

				packMsg := Message{Action: "open_pack"}
				for j := 0; j < packsPerClient; j++ {
					_ = json.NewEncoder(conn).Encode(packMsg)
					// Apenas lemos a resposta para garantir que o servidor não travou
					// ou retornou um erro inesperado.
					// A validação de sucesso ou falha (falta de cartas) é aceitável.
					_, err := readMessage(t, conn, 5*time.Second)
					if err != nil {
						t.Errorf("Client %d, Pack #%d: Failed to receive response: %v", id, j+1, err)
						return
					}
				}
			})
		}(i)
	}
	wg.Wait()
}


// TestStressPing aplica uma carga de múltiplos clientes enviando pings repetidamente
// e calcula a latência média de resposta para cada um.
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

				conn := connectAndRegister(t, clientName, make(map[string]int))
				defer conn.Close()

				encoder := json.NewEncoder(conn)
				decoder := json.NewDecoder(conn)
				pingMsg := Message{Action: "ping"}
				
				var totalLatency time.Duration
				var successfulPings int

				// Cada cliente envia múltiplos pings em sequência
				for j := 0; j < pingsPerClient; j++ {
					startTime := time.Now() // Marca o tempo de início

					if err := encoder.Encode(pingMsg); err != nil {
						t.Errorf("Request #%d: Failed to send ping: %v", j+1, err)
						return 
					}

					conn.SetReadDeadline(time.Now().Add(5 * time.Second))
					var response Message
					if err := decoder.Decode(&response); err != nil {
						t.Errorf("Request #%d: Failed to receive pong: %v", j+1, err)
						return
					}
					
					// Calcula a latência se a resposta for bem-sucedida
					if response.Action == "pong" {
						latency := time.Since(startTime)
						totalLatency += latency
						successfulPings++
					} else {
						t.Errorf("Request #%d: Expected action 'pong' but got '%s'", j+1, response.Action)
						return
					}
				}

				// Calcula e exibe a latência média para este cliente
				if successfulPings > 0 {
					averageLatency := totalLatency / time.Duration(successfulPings)
					t.Logf("Latência média para %s: %v", clientName, averageLatency)
				}
			})
		}(i)
	}

	// Espera que todos os clientes terminem
	wg.Wait()
}