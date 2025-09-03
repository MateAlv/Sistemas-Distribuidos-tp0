package common

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// BatchReader handles streaming CSV reading with batching
type BatchReader struct {
	scanner         *bufio.Scanner
	file            *os.File
	messageProtocol ProtocolConfig
	agency          string
	lineNumber      int
	totalRead       int
}

// NewBatchReader creates a new BatchReader for a CSV file
func NewBatchReader(filePath string, agency string, protocol ProtocolConfig) (*BatchReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %v", filePath, err)
	}

	return &BatchReader{
		scanner:         bufio.NewScanner(file),
		file:            file,
		agency:          agency,
		messageProtocol: protocol,
		lineNumber:      0,
		totalRead:       0,
	}, nil
}

// ReadNextBatch reads the next batch of bets from the file
// Returns nil slice when EOF is reached
func (br *BatchReader) ReadNextBatch() ([]*Bet, error) {
	var batch []*Bet

	for len(batch) < br.messageProtocol.BatchSize && br.scanner.Scan() {
		br.lineNumber++
		line := strings.TrimSpace(br.scanner.Text())

		bet, err := br.parseCSVLine(line)
		if err != nil {
			fmt.Printf("Skipping line %d: %v\n", br.lineNumber, err)
			continue
		}

		batch = append(batch, bet)
	}

	if err := br.scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file at line %d: %v", br.lineNumber, err)
	}

	br.totalRead += len(batch)
	return batch, nil
}

// parseCSVLine parses a CSV line into a Bet
func (br *BatchReader) parseCSVLine(line string) (*Bet, error) {
	line = strings.TrimPrefix(line, "\ufeff")

	reader := csv.NewReader(strings.NewReader(line))
	reader.Comma = ','

	records, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV line: %v", err)
	}

	if len(records) != 5 {
		return nil, fmt.Errorf("invalid CSV format: expected 5 fields, got %d", len(records))
	}

	number, err := strconv.Atoi(strings.TrimSpace(records[4]))
	if err != nil {
		return nil, fmt.Errorf("invalid number format: %v", err)
	}

	return NewBet(
		br.agency,                     // agency
		strings.TrimSpace(records[0]), // name
		strings.TrimSpace(records[1]), // lastname
		strings.TrimSpace(records[2]), // document
		strings.TrimSpace(records[3]), // birthdate
		number,                        // number
	), nil
}

// GetStats returns reading statistics
func (br *BatchReader) GetStats() (lineNumber, totalRead int) {
	return br.lineNumber, br.totalRead
}

// Close closes the underlying file
func (br *BatchReader) Close() error {
	if br.file != nil {
		return br.file.Close()
	}
	return nil
}

// SerializeBatch serializes multiple bets for network transmission
func (br *BatchReader) SerializeBatch(bets []*Bet) string {
	var serializedBets []string
	for _, bet := range bets {
		serializedBets = append(serializedBets, bet.Serialize())
	}
	return strings.Join(serializedBets, br.messageProtocol.BatchSeparator)
}
