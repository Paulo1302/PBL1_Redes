// File: server/state_validation_test.go
package server_test

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPlayCommandWithNoCards verifica se o servidor rejeita um jogador que tenta
// entrar na fila sem nenhuma carta.
func TestPlayCommandWithNoCards(t *testing.T) {
	// Conecta um cliente com inventário vazio
	emptyStock := make(map[string]int)
	conn := connectAndRegister(t, "NoCardsPlayer", emptyStock)
	defer conn.Close()

	// Tenta entrar na fila
	queueMsg := Message{Action: "queue_for_match"}
	_ = json.NewEncoder(conn).Encode(queueMsg)

	// Espera uma mensagem de erro
	response, err := readMessage(t, conn, 2*time.Second)
	if err != nil {
		t.Fatalf("Did not receive a response: %v", err)
	}
	if response.Action != "error" {
		t.Errorf("Expected action 'error', but got '%s'", response.Action)
	}
}

// TestMoveCommandWhenNotInGame verifica se um jogador não consegue fazer uma jogada
// se não estiver em uma partida ativa.
func TestMoveCommandWhenNotInGame(t *testing.T) {
	stock := map[string]int{"Rock": 1}
	conn := connectAndRegister(t, "IdlePlayer", stock)
	defer conn.Close()

	// Tenta fazer uma jogada enquanto está no estado "Idle"
	type PlayPayload struct {
		ChosenCard string `json:"chosen_card"`
	}
	playData, _ := json.Marshal(PlayPayload{ChosenCard: "Rock"})
	playMsg := Message{Action: "play", Data: playData}
	_ = json.NewEncoder(conn).Encode(playMsg)

	// Espera uma mensagem de erro
	response, err := readMessage(t, conn, 2*time.Second)
	if err != nil {
		t.Fatalf("Did not receive a response: %v", err)
	}
	if response.Action != "error" {
		t.Errorf("Expected action 'error', but got '%s'", response.Action)
	}
}