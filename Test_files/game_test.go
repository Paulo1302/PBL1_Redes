// File: Test_files/game_rules_test.go
package server_test

import (
	"PBL1_Redes/game"
	"testing"
)

// MockJogador é uma implementação de teste da interface game.Player.
type MockPlayer struct {
	Name       string
	Stock      map[game.Card]int
	PlayedCard game.Card
}

func (m *MockPlayer) GetName() string                { return m.Name }
func (m *MockPlayer) GetStock() map[game.Card]int   { return m.Stock }
func (m *MockPlayer) GetPlayedCard() game.Card     { return m.PlayedCard }
func (m *MockPlayer) SetPlayedCard(c game.Card)      { m.PlayedCard = c }
func (m *MockPlayer) RemoveCard(c game.Card)         { m.Stock[c]-- }
func (m *MockPlayer) AddCard(c game.Card)            { m.Stock[c]++ }

// TestDetermineWinner valida todos os cenários de vitória, derrota e empate.
func TestDetermineWinner(t *testing.T) {
	scenarios := []struct {
		name         string
		p1Card       game.Card
		p2Card       game.Card
		expectedWin  string // Nome do vencedor esperado
	}{
		{"Rock beats Scissors", game.Rock, game.Scissors, "Player1"},
		{"Paper beats Rock", game.Paper, game.Rock, "Player1"},
		{"Scissors beats Paper", game.Scissors, game.Paper, "Player1"},
		{"Draw with Rock", game.Rock, game.Rock, ""},
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