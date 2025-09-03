package common

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID              string
	ServerAddress   string
	MessageProtocol ProtocolConfig
}

// ProtocolConfig holds message protocol configuration
type ProtocolConfig struct {
	BatchSize               int
	FieldSeparator          string
	BatchSeparator          string
	MessageDelimiter        string
	SuccessResponse         string
	FailureResponse         string
	ProtocolFinishedMessage string
	ProtocolQueryWinners    string
	ProtocolWinnersResponse string
}

// Client Entity that encapsulates how
type Client struct {
	config      ClientConfig
	conn        net.Conn
	sigChan     chan os.Signal
	batchReader *BatchReader
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig, csvFilePath string) (*Client, error) {
	reader, err := NewBatchReader(csvFilePath, config.ID, config.MessageProtocol)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch reader: %v", err)
	}

	client := &Client{
		config:      config,
		sigChan:     make(chan os.Signal, 1),
		batchReader: reader,
	}

	signal.Notify(client.sigChan, syscall.SIGTERM)
	return client, nil
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}
	c.conn = conn
	return nil
}

// SendBatch sends a batch of bets to the server
func (c *Client) SendBatch(bets []*Bet) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()

	batchData := c.batchReader.SerializeBatch(bets)
	message := batchData + c.config.MessageProtocol.MessageDelimiter

	_, err := c.conn.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to send batch: %v", err)
	}

	response, err := bufio.NewReader(c.conn).ReadString('\n')
	if err != nil {
		log.Errorf("action: apuesta_enviada | result: fail | cantidad: %d | error: %v", len(bets), err)
		return err
	}

	response = strings.TrimSpace(response)

	if response == c.config.MessageProtocol.SuccessResponse {
		log.Infof("action: apuesta_enviada | result: success | cantidad: %d", len(bets))
		return nil
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | cantidad: %d | server_response: %s", len(bets), response)
		return fmt.Errorf("server rejected batch: %s", response)
	}
}

// StartClientLoop processes CSV file in batches using internal reader and receives the winners list at the end
func (c *Client) StartClientLoop() error {
	// Create persistent connection
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()

	go func() {
		<-c.sigChan
		log.Infof("action: sigterm_received | result: success | client_id: %v", c.config.ID)
		c.GracefulShutdown()
	}()

	batchNumber := 0

	// Process all batches on same connection
	for {
		batch, err := c.batchReader.ReadNextBatch()
		if err != nil {
			return fmt.Errorf("failed to read batch: %v", err)
		}

		if len(batch) == 0 {
			break
		}

		batchNumber++
		if err := c.sendBatch(batch); err != nil {
			return fmt.Errorf("failed to send batch %d: %v", batchNumber, err)
		}
	}

	// Send FINISHED and get winners on same connection
	winners, err := c.sendFinishedAndGetWinners()
	if err != nil {
		return fmt.Errorf("failed to get winners: %v", err)
	}

	// Log winners count
	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))

	return nil
}

func (c *Client) SendFinishedNotification() error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()

	message := c.config.MessageProtocol.ProtocolFinishedMessage + c.config.MessageProtocol.MessageDelimiter

	if _, err := c.conn.Write([]byte(message)); err != nil {
		return fmt.Errorf("failed to send finished notification: %v", err)
	}

	// Read response
	response := make([]byte, 1024)
	n, err := c.conn.Read(response)
	if err != nil {
		return fmt.Errorf("failed to read finished response: %v", err)
	}

	responseStr := strings.TrimSpace(string(response[:n]))
	if responseStr != c.config.MessageProtocol.SuccessResponse {
		return fmt.Errorf("server rejected finished notification: %s", responseStr)
	}

	log.Infof("action: finished_notification_sent | result: success")
	return nil
}

func (c *Client) sendBatch(bets []*Bet) error {
	batchData := c.batchReader.SerializeBatch(bets)
	message := batchData + c.config.MessageProtocol.MessageDelimiter

	_, err := c.conn.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to send batch: %v", err)
	}

	// Read response
	response, err := bufio.NewReader(c.conn).ReadString('\n')
	if err != nil {
		return err
	}

	response = strings.TrimSpace(response)
	if response != c.config.MessageProtocol.SuccessResponse {
		return fmt.Errorf("server rejected batch: %s", response)
	}

	log.Infof("action: apuesta_enviada | result: success | cantidad: %d", len(bets))
	return nil
}

func (c *Client) sendFinishedAndGetWinners() ([]string, error) {
	// Send FINISHED message to server
	message := c.config.MessageProtocol.ProtocolFinishedMessage +
		c.config.MessageProtocol.MessageDelimiter +
		c.config.MessageProtocol.MessageDelimiter

	if _, err := c.conn.Write([]byte(message)); err != nil {
		return nil, fmt.Errorf("failed to send finished: %v", err)
	}

	// Read winners response
	response, err := bufio.NewReader(c.conn).ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read winners: %v", err)
	}

	response = strings.TrimSpace(response)

	// Parse winners: "WINNERS:dni1~dni2~dni3" or "WINNERS:"
	if !strings.HasPrefix(response, c.config.MessageProtocol.ProtocolWinnersResponse) {
		return nil, fmt.Errorf("unexpected response format: %s", response)
	}

	winnersData := strings.TrimPrefix(response, c.config.MessageProtocol.ProtocolWinnersResponse)
	if winnersData == "" {
		return []string{}, nil // No winners
	}

	// Split by batch separator (assumed to be "~")
	winners := strings.Split(winnersData, c.config.MessageProtocol.BatchSeparator)

	// Filter out any empty strings that may result from splitting
	var filteredWinners []string
	for _, winner := range winners {
		winner = strings.TrimSpace(winner)
		if winner != "" {
			filteredWinners = append(filteredWinners, winner)
		}
	}

	return filteredWinners, nil
}

// GracefulShutdown makes sure all resources are released properly when the client is shutting down
func (c *Client) GracefulShutdown() {
	log.Infof("action: client_shutdown | result: in_progress | client_id: %v", c.config.ID)

	if c.conn != nil {
		log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
		c.conn.Close()
	}

	if c.batchReader != nil {
		c.batchReader.Close()
	}

	log.Infof("action: client_shutdown | result: success | client_id: %v", c.config.ID)
	os.Exit(0)
}
