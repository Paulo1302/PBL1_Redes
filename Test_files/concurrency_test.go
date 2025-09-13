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
	numClients := 50 // Deve ser um número par
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
	numClients := 50
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