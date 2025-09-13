// File: server/game_flow_test.go
package server_test

import (
	// CORREÇÃO: Removida a importação "PBL1_Redes-card_game/server" que não estava em uso.
	"encoding/json"
	"testing"
	"time"
)

// TestFullGameFlowWinLoss valida o fluxo completo de um jogo de uma rodada,
// incluindo a jogada, o resultado e a correta atualização do inventário.
func TestFullGameFlowWinLoss(t *testing.T) {
	stock := map[string]int{"Rock": 1, "Paper": 1, "Scissors": 1}
	client1Conn := connectAndRegister(t, "PlayerWinner", stock)
	defer client1Conn.Close()
	client2Conn := connectAndRegister(t, "PlayerLoser", stock)
	defer client2Conn.Close()

	// 1. Ambos os clientes entram na fila
	queueMsg := Message{Action: "queue_for_match"}
	_ = json.NewEncoder(client1Conn).Encode(queueMsg)
	_ = json.NewEncoder(client2Conn).Encode(queueMsg)

	// 2. Verificam se foram pareados
	msg1, err := readMessage(t, client1Conn, 5*time.Second) // queue_success
	if err != nil || msg1.Action != "queue_success" {
		t.Fatalf("Winner did not receive queue_success: %v", err)
	}
	msg1, err = readMessage(t, client1Conn, 5*time.Second) // matched
	if err != nil || msg1.Action != "matched" {
		t.Fatalf("Winner did not get matched: %v", err)
	}

	// Apenas para consumir as mensagens do segundo cliente
	readMessage(t, client2Conn, 5*time.Second) // queue_success
	readMessage(t, client2Conn, 5*time.Second) // matched

	// 3. Aguardam o prompt para jogar
	readMessage(t, client1Conn, 5*time.Second) // game_prompt_play
	readMessage(t, client2Conn, 5*time.Second) // game_prompt_play

	// 4. Jogadores fazem suas jogadas (Vencedor joga Papel, Perdedor joga Pedra)
	type PlayPayload struct {
		ChosenCard string `json:"chosen_card"`
	}
	playDataWinner, _ := json.Marshal(PlayPayload{ChosenCard: "Paper"})
	playMsgWinner := Message{Action: "play", Data: playDataWinner}
	_ = json.NewEncoder(client1Conn).Encode(playMsgWinner)

	playDataLoser, _ := json.Marshal(PlayPayload{ChosenCard: "Rock"})
	playMsgLoser := Message{Action: "play", Data: playDataLoser}
	_ = json.NewEncoder(client2Conn).Encode(playMsgLoser)

	// 5. Verificam o resultado de "game_over"
	gameOverMsgWinner, err := readMessage(t, client1Conn, 5*time.Second)
	if err != nil || gameOverMsgWinner.Action != "game_over" {
		t.Fatalf("Winner did not receive game_over. Got %s: %v", gameOverMsgWinner.Action, err)
	}
	gameOverMsgLoser, err := readMessage(t, client2Conn, 5*time.Second)
	if err != nil || gameOverMsgLoser.Action != "game_over" {
		t.Fatalf("Loser did not receive game_over. Got %s: %v", gameOverMsgLoser.Action, err)
	}

	// 6. Validam o conteúdo do payload de game_over e o novo inventário
	type GameOverPayload struct {
		Content    string         `json:"content"`
		FinalStock map[string]int `json:"final_stock"`
	}
	var winnerPayload, loserPayload GameOverPayload
	json.Unmarshal(gameOverMsgWinner.Data, &winnerPayload)
	json.Unmarshal(gameOverMsgLoser.Data, &loserPayload)

	// Valida o conteúdo da mensagem
	if winnerPayload.Content != "You won the round!" {
		t.Errorf("Winner received wrong message: '%s'", winnerPayload.Content)
	}
	if loserPayload.Content != "You lost the round!" {
		t.Errorf("Loser received wrong message: '%s'", loserPayload.Content)
	}

	// Valida o inventário final (Vencedor deve ter pego a carta "Rock" do perdedor)
	if val := winnerPayload.FinalStock["Rock"]; val != 2 {
		t.Errorf("Winner's final stock of Rock is wrong. Expected 2, got %d. Stock: %v", val, winnerPayload.FinalStock)
	}
	if _, ok := loserPayload.FinalStock["Rock"]; ok {
		t.Errorf("Loser's Rock should have been removed. Stock: %v", loserPayload.FinalStock)
	}
}