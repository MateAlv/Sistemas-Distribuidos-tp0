package common

import (
	"fmt"
	"strconv"
	"strings"
)

// Bet represents lottery bet data
type Bet struct {
	Agency    string
	Name      string
	LastName  string
	Document  string
	Birthdate string
	Number    int
}

// NewBet creates a new Bet instance
func NewBet(agency, name, lastName, document, birthdate string, number int) *Bet {
	return &Bet{
		Agency:    agency,
		Name:      name,
		LastName:  lastName,
		Document:  document,
		Birthdate: birthdate,
		Number:    number,
	}
}

// Serialize converts the Bet to custom protocol string
func (bet *Bet) Serialize() string {
	return fmt.Sprintf("%s;%s;%s;%s;%s;%d",
		bet.Agency, bet.Name, bet.LastName, bet.Document, bet.Birthdate, bet.Number)
}

// DeserializeBet deserializes custom protocol string to Bet
func DeserializeBet(data string) (*Bet, error) {
	parts := strings.Split(data, ";")
	if len(parts) != 6 {
		return nil, fmt.Errorf("invalid bet format: expected 6 parts, got %d", len(parts))
	}

	number, err := strconv.Atoi(parts[5])
	if err != nil {
		return nil, fmt.Errorf("invalid number format: %w", err)
	}

	return &Bet{
		Agency:    parts[0],
		Name:      parts[1],
		LastName:  parts[2],
		Document:  parts[3],
		Birthdate: parts[4],
		Number:    number,
	}, nil
}

// IsValid validates that all fields are present
func (bet *Bet) IsValid() bool {
	return bet.Agency != "" &&
		bet.Name != "" &&
		bet.LastName != "" &&
		bet.Document != "" &&
		bet.Birthdate != "" &&
		bet.Number > 0
}
