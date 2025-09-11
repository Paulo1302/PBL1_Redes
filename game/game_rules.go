// File: game/game_rules.go
package game

import (
	"math/rand"
)

// Card represents the game options.
type Card string

const (
	Rock    Card = "Rock"
	Paper   Card = "Paper"
	Scissors Card = "Scissors"
)

// Player is an interface that represents a contract.
// The Client type in the server package must implement these methods.
type Player interface {
	GetName() string
	GetStock() map[Card]int
	GetPlayedCard() Card
	SetPlayedCard(c Card)
	RemoveCard(c Card)
	AddCard(c Card)
}

// PlayMove selects a random card from the player's stock for automated play.
func PlayMove(p Player) {
	stock := p.GetStock()
	if len(stock) == 0 {
		p.SetPlayedCard("") // The player has no more cards to play.
		return
	}

	availableCards := make([]Card, 0, len(stock))
	for card, quantity := range stock {
		for i := 0; i < quantity; i++ {
			availableCards = append(availableCards, card)
		}
	}
	
	if len(availableCards) > 0 {
		chosenCard := availableCards[rand.Intn(len(availableCards))]
		p.SetPlayedCard(chosenCard)
		p.RemoveCard(chosenCard)
	}
}

// DetermineWinner determines the winner of the round.
func DetermineWinner(p1, p2 Player) (Player, Player) {
	c1 := p1.GetPlayedCard()
	c2 := p2.GetPlayedCard()

	if c1 == "" || c2 == "" {
		return nil, nil // A player didn't play a card
	}

	if c1 == c2 {
		return nil, nil // Draw
	}

	switch c1 {
	case Rock:
		if c2 == Scissors {
			return p1, p2
		}
	case Paper:
		if c2 == Rock {
			return p1, p2
		}
	case Scissors:
		if c2 == Paper {
			return p1, p2
		}
	}
	return p2, p1 // If none of the above, p2 wins
}

// SwapCards moves the played card from the loser to the winner's stock.
func SwapCards(winner, loser Player) {
	if winner != nil && loser != nil {
		winner.AddCard(loser.GetPlayedCard())
	}
}

// PlayRound orchestrates a single automated round of the game and returns the result.
func PlayRound(p1, p2 Player) (Player, Player) {
	PlayMove(p1)
	PlayMove(p2)
	return DetermineWinner(p1, p2)
}