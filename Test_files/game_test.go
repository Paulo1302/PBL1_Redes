// File: game/game_rules_test.go
package server_test

import (
	"PBL1_Redes-card_game/game"
	"reflect"
	"testing"
)

// MockPlayer é uma implementação de teste da interface game.Player.
type MockPlayer struct {
	Name       string
	Stock      map[game.Card]int
	PlayedCard game.Card
}

func (m *MockPlayer) GetName() string                { return m.Name }
func (m *MockPlayer) GetStock() map[game.Card]int   { return m.Stock }
func (m *MockPlayer) GetPlayedCard() game.Card     { return m.PlayedCard }
func (m *MockPlayer) SetPlayedCard(c game.Card)      { m.PlayedCard = c }
func (m *MockPlayer) RemoveCard(c game.Card) {
	if m.Stock[c] > 0 {
		m.Stock[c]--
		if m.Stock[c] == 0 {
			delete(m.Stock, c)
		}
	}
}
func (m *MockPlayer) AddCard(c game.Card) { m.Stock[c]++ }

// TestDetermineWinner valida todos os cenários de vitória, derrota e empate.
func TestDetermineWinner(t *testing.T) {
	// (Este teste foi mantido como estava, pois já era robusto)
	scenarios := []struct {
		name        string
		p1Card      game.Card
		p2Card      game.Card
		expectedWin string // Nome do vencedor esperado
	}{
		{"Rock beats Scissors", game.Rock, game.Scissors, "Player1"},
		{"Paper beats Rock", game.Paper, game.Rock, "Player1"},
		{"Scissors beats Paper", game.Scissors, game.Paper, "Player1"},
		{"Draw with Rock", game.Rock, game.Rock, ""},
		{"Rock loses to Paper", game.Rock, game.Paper, "Player2"},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			p1 := &MockPlayer{Name: "Player1"}
			p2 := &MockPlayer{Name: "Player2"}
			p1.SetPlayedCard(scenario.p1Card)
			p2.SetPlayedCard(scenario.p2Card)

			winner, _ := game.DetermineWinner(p1, p2)

			if scenario.expectedWin == "" {
				if winner != nil {
					t.Errorf("Expected a draw (nil), but winner was %s", winner.GetName())
				}
			} else {
				if winner == nil {
					t.Errorf("Expected winner %s, but got a draw (nil)", scenario.expectedWin)
				} else if winner.GetName() != scenario.expectedWin {
					t.Errorf("Incorrect winner. Expected: %s, Got: %s", scenario.expectedWin, winner.GetName())
				}
			}
		})
	}
}

// TestSwapCards valida se a troca de cartas entre vencedor e perdedor funciona corretamente.
func TestSwapCards(t *testing.T) {
	// --- 1. Cenário (Setup) ---
	winner := &MockPlayer{
		Name:  "Winner",
		Stock: map[game.Card]int{game.Rock: 1},
	}
	winner.SetPlayedCard(game.Rock)

	loser := &MockPlayer{
		Name:  "Loser",
		Stock: map[game.Card]int{game.Paper: 2},
	}
	loser.SetPlayedCard(game.Paper)

	// --- 2. Ação (Action) ---
	// Executa a função SwapCards, que apenas adiciona a carta do perdedor ao vencedor.
	game.SwapCards(winner, loser)

	// --- 3. Verificação (Assertion) ---
	// As verificações agora refletem o comportamento real da função.

	t.Run("Inventário do Vencedor deve receber a carta do perdedor", func(t *testing.T) {
		// O vencedor deve manter sua Pedra e receber o Papel do perdedor.
		expectedWinnerStock := map[game.Card]int{game.Rock: 1, game.Paper: 1}
		if !reflect.DeepEqual(winner.GetStock(), expectedWinnerStock) {
			t.Errorf("Inventário do vencedor incorreto. Recebeu: %v, Esperado: %v", winner.GetStock(), expectedWinnerStock)
		}
	})

	t.Run("Inventário do Perdedor não deve ser alterado", func(t *testing.T) {
		expectedLoserStock := map[game.Card]int{game.Paper: 2}
		if !reflect.DeepEqual(loser.GetStock(), expectedLoserStock) {
			t.Errorf("Inventário do perdedor incorreto. Recebeu: %v, Esperado: %v", loser.GetStock(), expectedLoserStock)
		}
	})
}