package common

import (
    "fmt"
    "strconv"
    "strings"
)

// Bet represents lottery bet data
type Bet struct {
    Name      	string
    LastName  	string
    Document  	string
    Birthdate 	string
    Number    	int
}

// NewBet creates a new Bet instance
func NewBet(name, lastName, document, birthdate string, number int) *Bet {
    return &Bet{
        Name:      	name,
        LastName:  	lastName,
        Document:  	document,
        Birthdate: 	birthdate,
        Number:    	number,
    }
}

// Serialize converts the Bet to custom protocol string
func (lt *Bet) Serialize() string {
    return fmt.Sprintf("%s|%s|%s|%s|%d", 
        lt.Name, lt.LastName, lt.Document, lt.Birthdate, lt.Number)
}

// DeserializeBet deserializes custom protocol string to Bet
func DeserializeBet(data string) (*Bet, error) {
    parts := strings.Split(data, "|")
    if len(parts) != 5 {
        return nil, fmt.Errorf("invalid bet format: expected 5 parts, got %d", len(parts))
    }
    
    number, err := strconv.Atoi(parts[4])
    if err != nil {
        return nil, fmt.Errorf("invalid number format: %w", err)
    }
    
    return &Bet{
        Name:      	parts[0],
        LastName:  	parts[1],
        Document:  	parts[2],
        Birthdate: 	parts[3],
        Number:    	number,
    }, nil
}

// String returns a string representation of the bet
func (lt *Bet) String() string {
    return fmt.Sprintf("Bet{Name: %s, LastName: %s, Document: %s, Birthdate: %s, Number: %d}",
        lt.Name, lt.LastName, lt.Document, lt.Birthdate, lt.Number)
}

// IsValid validates that all fields are present
func (lt *Bet) IsValid() bool {
    return lt.Name != "" && 
           lt.LastName != "" && 
           lt.Document != "" && 
           lt.Birthdate != "" && 
           lt.Number > 0
}