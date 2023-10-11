package hangman

import (
	"errors"
	"math/rand"
	"strings"
	"unicode"
)

type hangman struct {
	attemptedLetters [26]bool
	chosenWord       string
	currentState     string
	turnCount        uint8
}

type Hangman interface {
	Guess(guess byte) (string, error)
	IsOver() bool
	HasWon() bool
	GetCurrentState() string
	GetWord() string
}

var theWords = []string {
	"activated",
	"activates",
}

func New() Hangman {
	wordIndex := rand.Intn(len(theWords))
	chosenWord := theWords[wordIndex]

	currentState := strings.Repeat("_ ", len(chosenWord))

	return &hangman{
		chosenWord: chosenWord,
		currentState: currentState,
	}
}

func (h *hangman) Guess(guess byte) (nextState string, err error) {
	if (guess >= 'A' && guess <= 'Z') {
		guess = byte(unicode.ToLower(rune(guess)))
	}

	if (guess < 'a' || guess > 'z') {
		return h.currentState, errors.New("invalid character")
	}

	if h.attemptedLetters[guess - 97] {
		return h.currentState, errors.New("already attempted letter")
	}

	h.calculateNextState(guess)

	return h.currentState, nil
}

func (h *hangman) IsOver() bool {
	return h.turnCount > 5 || h.HasWon()
}

func (h *hangman) HasWon() bool {
	return !strings.Contains(h.currentState, "_")
}

func (h *hangman) GetCurrentState()  string {
	return h.currentState
}

func (h *hangman) GetWord() string {
	return h.chosenWord
}

func (h *hangman) calculateNextState(guess byte) {
	h.attemptedLetters[guess - 97] = true

	nextState := ""
	didRevealAnything := false
	for i, character := range h.chosenWord {
		if h.currentState[i*2] != '_' {
			nextState += string(character) + " "
			continue
		}

		if character == rune(guess) {
			nextState += string(character)
			didRevealAnything = true
		} else {
			nextState += "_"
		}

		nextState += " "
	}

	if !didRevealAnything {
		h.turnCount++
	}

	h.currentState = nextState
}