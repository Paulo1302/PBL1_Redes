package game

import (
	"math/rand"
	"time"
)

// Carta representa as opções do jogo.
type Carta string

const (
	Pedra   Carta = "Pedra"
	Papel   Carta = "Papel"
	Tesoura Carta = "Tesoura"
)

// Jogador é uma interface que representa um contrato.
// O tipo Usuario em main.go deve implementar esses métodos.
type Jogador interface {
	GetNome() string
	GetEstoque() map[Carta]int
	GetCartaJogada() Carta
	SetCartaJogada(c Carta)
	RemoverCarta(c Carta)
	AdicionarCarta(c Carta)
}

// Jogar seleciona uma carta aleatória do estoque do jogador.
func Jogar(j Jogador) {
	estoque := j.GetEstoque()
	if len(estoque) == 0 {
		j.SetCartaJogada("") // O jogador não tem mais cartas para jogar.
		return
	}

	rand.Seed(time.Now().UnixNano())
	cartasDisponiveis := make([]Carta, 0, len(estoque))
	for carta, quantidade := range estoque {
		for i := 0; i < quantidade; i++ {
			cartasDisponiveis = append(cartasDisponiveis, carta)
		}
	}
	
	if len(cartasDisponiveis) > 0 {
		cartaEscolhida := cartasDisponiveis[rand.Intn(len(cartasDisponiveis))]
		j.SetCartaJogada(cartaEscolhida)
		j.RemoverCarta(cartaEscolhida)
	}
}

// Vencedor determina o vencedor da partida.
func Vencedor(j1, j2 Jogador) (Jogador, Jogador) {
	c1 := j1.GetCartaJogada()
	c2 := j2.GetCartaJogada()

	if c1 == "" || c2 == "" {
		return nil, nil
	}

	if c1 == c2 {
		return nil, nil // Empate
	}

	switch c1 {
	case Pedra:
		if c2 == Tesoura {
			return j1, j2
		}
	case Papel:
		if c2 == Pedra {
			return j1, j2
		}
	case Tesoura:
		if c2 == Papel {
			return j1, j2
		}
	}
	return j2, j1
}

// TrocarCartas move a carta jogada do perdedor para o estoque do vencedor.
func TrocarCartas(vencedor, perdedor Jogador) {
	if vencedor != nil && perdedor != nil {
		vencedor.AdicionarCarta(perdedor.GetCartaJogada())
	}
}

// JogaRodada orquestra uma única rodada do jogo e retorna o resultado.
func JogaRodada(j1, j2 Jogador) (Jogador, Jogador) {
	Jogar(j1)
	Jogar(j2)
	return Vencedor(j1, j2)
}