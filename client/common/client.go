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

// StartClientLoop processes CSV file in batches using internal reader
func (c *Client) StartClientLoop() error {
	go func() {
		<-c.sigChan
		log.Infof("action: sigterm_received | result: success | client_id: %v", c.config.ID)
		c.GracefulShutdown()
	}()

	batchNumber := 0

	// Process batches using internal reader
	for {
		batch, err := c.batchReader.ReadNextBatch()
		if err != nil {
			return fmt.Errorf("failed to read batch: %v", err)
		}

		if len(batch) == 0 {
			break
		}

		batchNumber++
		log.Infof("action: batch_prepared | result: success | batch_number: %d | size: %d",
			batchNumber, len(batch))

		if err := c.SendBatch(batch); err != nil {
			return fmt.Errorf("failed to send batch %d: %v", batchNumber, err)
		}
	}

	lineNumber, totalRead := c.batchReader.GetStats()
	log.Infof("action: file_processed | result: success | total_batches: %d | total_bets: %d | lines_read: %d",
		batchNumber, totalRead, lineNumber)

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

func (c *Client) sendMessageAndReceiveResponse(message string) (string, error) {
	err := c.createClientSocket()
	if err != nil {
		return "", fmt.Errorf("failed to create socket: %v", err)
	}
	defer c.conn.Close()

	// Send message
	fullMessage := message + c.config.MessageProtocol.MessageDelimiter
	if _, err := c.conn.Write([]byte(fullMessage)); err != nil {
		return "", fmt.Errorf("failed to send message: %v", err)
	}

	// Read response in chunks until delimiter
	var response strings.Builder
	buffer := make([]byte, 256) // Reasonable chunk size
	delimiter := c.config.MessageProtocol.MessageDelimiter

	for {
		n, err := c.conn.Read(buffer)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %v", err)
		}

		chunk := string(buffer[:n])
		response.WriteString(chunk)

		// Check if we have complete message
		if strings.Contains(response.String(), delimiter) {
			break
		}
	}

	// Extract message before delimiter
	result := response.String()
	delimiterPos := strings.Index(result, delimiter)
	return result[:delimiterPos], nil
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
