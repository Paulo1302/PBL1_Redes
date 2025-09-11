package Test_files_game_test // Pacote de teste externo para o pacote 'server'

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"
)

// Como este é um pacote de teste externo, as structs não exportadas do pacote 'server'
// (como Message e PairedMessage) precisam ser redeclaradas aqui para que o
// cliente de teste possa codificar e decodificar as mensagens JSON.
type Message struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type PairedMessage struct {
	Content string `json:"content"`
}

// isServerRunning verifica se o servidor já está rodando na porta esperada.
func isServerRunning(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// TestFullGameFlow simula uma partida completa entre dois clientes.
func TestFullGameFlow(t *testing.T) {
	if !isServerRunning("localhost:8080") {
		t.Skip("Servidor não está em execução. Execute 'go run serverStart.go' em outro terminal para rodar este teste.")
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Cliente 1
	go func() {
		defer wg.Done()
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			t.Errorf("Cliente 1 falhou ao conectar: %v", err)
			return
		}
		defer conn.Close()
		decoder := json.NewDecoder(conn)

		// 1. Espera mensagem de pareamento
		var matchedMsg Message
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		err = decoder.Decode(&matchedMsg)
		if err != nil || matchedMsg.Action != "matched" {
			t.Errorf("Cliente 1 falhou ao receber mensagem de pareamento: %v", err)
			return
		}

		// 2. Espera resultado da primeira rodada
		var roundResultMsg Message
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		err = decoder.Decode(&roundResultMsg)
		if err != nil || roundResultMsg.Action != "game_round_result" {
			t.Errorf("Cliente 1 falhou ao receber resultado da rodada: %v", err)
			return
		}
		t.Logf("Cliente 1 recebeu resultado da rodada: %s", string(roundResultMsg.Data))
	}()

	// Cliente 2
	go func() {
		defer wg.Done()
		conn, err := net.Dial("tcp", "localhost:8080")
		if err != nil {
			t.Errorf("Cliente 2 falhou ao conectar: %v", err)
			return
		}
		defer conn.Close()
		decoder := json.NewDecoder(conn)

		// 1. Espera mensagem de pareamento
		var matchedMsg Message
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		err = decoder.Decode(&matchedMsg)
		if err != nil || matchedMsg.Action != "matched" {
			t.Errorf("Cliente 2 falhou ao receber mensagem de pareamento: %v", err)
			return
		}

		// 2. Espera resultado da primeira rodada
		var roundResultMsg Message
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		err = decoder.Decode(&roundResultMsg)
		if err != nil || roundResultMsg.Action != "game_round_result" {
			t.Errorf("Cliente 2 falhou ao receber resultado da rodada: %v", err)
			return
		}
		t.Logf("Cliente 2 recebeu resultado da rodada: %s", string(roundResultMsg.Data))
	}()

	wg.Wait()
}

// TestStressServerWithWaitGroup aplica uma carga de múltiplos clientes simultâneos.
func TestStressServerWithWaitGroup(t *testing.T) {
	if !isServerRunning("localhost:8080") {
		t.Skip("Servidor não está em execução. Execute 'go run serverStart.go' em outro terminal para rodar este teste.")
	}

	var wg sync.WaitGroup
	numClients := 50 // Use um número par para garantir que todos sejam pareados.

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", "localhost:8080")
			if err != nil {
				t.Errorf("[Cliente %d] falhou ao conectar: %v", id, err)
				return
			}
			defer conn.Close()

			decoder := json.NewDecoder(conn)
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))

			// Cada cliente deve receber a notificação de que foi pareado.
			var matchedMsg Message
			err = decoder.Decode(&matchedMsg)
			if err != nil {
				t.Errorf("[Cliente %d] não recebeu a mensagem de pareamento a tempo: %v", id, err)
				return
			}

			if matchedMsg.Action != "matched" {
				t.Errorf("[Cliente %d] esperava ação 'matched', mas recebeu '%s'", id, matchedMsg.Action)
			}
		}(i)
	}

	// A função principal do teste espera aqui até que todas as goroutines de clientes terminem.
	wg.Wait()
	t.Logf("%d clientes conectados e pareados com sucesso no teste de estresse.", numClients)
}