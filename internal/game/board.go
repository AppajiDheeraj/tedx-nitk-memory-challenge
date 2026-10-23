package game

import (
	"errors"
	"math/rand"
	"strings"
)

type Layout struct {
	Difficulty string
	Rows       int
	Columns    int
}

func GetLayout(difficulty string) (Layout, error) {
	switch strings.ToLower(strings.TrimSpace(difficulty)) {
	case "easy":
		return Layout{"easy", 4, 4}, nil
	case "medium":
		return Layout{"medium", 4, 5}, nil
	case "hard":
		return Layout{"hard", 5, 6}, nil
	default:
		return Layout{}, errors.New(
			"difficulty must be easy, medium, or hard",
		)
	}
}

func GenerateBoard(rows, columns int) ([]Card, int, error) {
	if !((rows == 4 && columns == 4) ||
		(rows == 4 && columns == 5) ||
		(rows == 5 && columns == 6)) {
		return nil, 0, errors.New("invalid board dimensions")
	}

	fruits := []string{
		"apple", "banana", "grapes", "orange",
		"strawberry", "watermelon", "cherry", "kiwi",
		"mango", "pineapple", "pear", "peach",
		"lemon", "blueberry", "coconut",
	}

	totalCards := rows * columns
	totalPairs := totalCards / 2
	cards := make([]Card, 0, totalCards)

	for i := 0; i < totalPairs; i++ {
		cards = append(cards,
			Card{PairID: i + 1, Value: fruits[i]},
			Card{PairID: i + 1, Value: fruits[i]},
		)
	}

	rand.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})

	for i := range cards {
		cards[i].Position = i
	}

	return cards, totalPairs, nil
}
