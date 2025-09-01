package common

import (
    "fmt"
    "strconv"
    "strings"
)

// LotteryTicket represents lottery bet data
type LotteryTicket struct {
    Name      string
    LastName  string
    ID  string
    Birthdate string
    Number    int
}

// NewLotteryTicket creates a new LotteryTicket instance
func NewLotteryTicket(name, lastName, id, birthdate string, number int) *LotteryTicket {
    return &LotteryTicket{
        Name:      name,
        LastName:  lastName,
        ID:  id,
        Birthdate: birthdate,
        Number:    number,
    }
}

// Serialize converts the LotteryTicket to custom protocol string
func (lt *LotteryTicket) Serialize() string {
    return fmt.Sprintf("%s|%s|%s|%s|%d", 
        lt.Name, lt.LastName, lt.ID, lt.Birthdate, lt.Number)
}

// DeserializeLotteryTicket deserializes custom protocol string to LotteryTicket
func DeserializeLotteryTicket(data string) (*LotteryTicket, error) {
    parts := strings.Split(data, "|")
    if len(parts) != 5 {
        return nil, fmt.Errorf("invalid ticket format: expected 5 parts, got %d", len(parts))
    }
    
    number, err := strconv.Atoi(parts[4])
    if err != nil {
        return nil, fmt.Errorf("invalid number format: %w", err)
    }
    
    return &LotteryTicket{
        Name:      parts[0],
        LastName:  parts[1],
        ID:  parts[2],
        Birthdate: parts[3],
        Number:    number,
    }, nil
}

// String returns a string representation of the ticket
func (lt *LotteryTicket) String() string {
    return fmt.Sprintf("LotteryTicket{Name: %s, LastName: %s, ID: %s, Birthdate: %s, Number: %d}",
        lt.Name, lt.LastName, lt.ID, lt.Birthdate, lt.Number)
}

// IsValid validates that all fields are present
func (lt *LotteryTicket) IsValid() bool {
    return lt.Name != "" && 
           lt.LastName != "" && 
           lt.ID != "" && 
           lt.Birthdate != "" && 
           lt.Number > 0
}