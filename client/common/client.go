package common

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
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
	BatchSize              int
	BatchSeparator         string
	MessageDelimiter       string
	SuccessResponse        string
	ProtocolFinishedHeader string
	ProtocolFinishedBody   string
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
		log.Criticalf("action: connect | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
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

	err := writeAll(c.conn, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send batch: %v", err)
	}

	reader := bufio.NewReader(c.conn)
	hdr, body, err := readTwoLines(reader, c.config.MessageProtocol.MessageDelimiter[0])
	if err != nil {
		log.Errorf("action: apuesta_enviada | result: fail | cantidad: %d | error: %v", len(bets), err)
		return err
	}
	if !strings.HasPrefix(hdr, "R:") || body != c.config.MessageProtocol.SuccessResponse {
		log.Errorf("action: apuesta_enviada | result: fail | cantidad: %d | server_response: %s|%s", len(bets), hdr, body)
		return fmt.Errorf("server rejected batch: %s|%s", hdr, body)
	}
	log.Infof("action: apuesta_enviada | result: success | cantidad: %d", len(bets))
	return nil

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

func (c *Client) sendBatch(bets []*Bet) error {
	batchData := c.batchReader.SerializeBatch(bets)
	message := batchData + c.config.MessageProtocol.MessageDelimiter

	err := writeAll(c.conn, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send batch: %v", err)
	}

	// Read response
	reader := bufio.NewReader(c.conn)
	hdr, body, err := readTwoLines(reader, c.config.MessageProtocol.MessageDelimiter[0])
	if err != nil {
		return err
	}
	if !strings.HasPrefix(hdr, "R:") || body != c.config.MessageProtocol.SuccessResponse {
		log.Infof("action: apuesta_enviada | result: failure | cantidad: %d", len(bets))
		return fmt.Errorf("server rejected batch: %s|%s", hdr, body)
	}

	log.Infof("action: apuesta_enviada | result: success | cantidad: %d", len(bets))
	return nil
}

func (c *Client) sendFinishedAndGetWinners() ([]string, error) {
	// Send FINISHED (2 líneas): F:1\nFINISHED\n
	message := c.config.MessageProtocol.ProtocolFinishedHeader + "1" + c.config.MessageProtocol.MessageDelimiter +
		c.config.MessageProtocol.ProtocolFinishedBody + c.config.MessageProtocol.MessageDelimiter

	if err := writeAll(c.conn, []byte(message)); err != nil {
		return nil, fmt.Errorf("failed to send finished: %v", err)
	}

	// Read winners response: W:<count>\n<body>\n
	reader := bufio.NewReader(c.conn)
	hdr, body, err := readTwoLines(reader, c.config.MessageProtocol.MessageDelimiter[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read winners: %v", err)
	}

	if !strings.HasPrefix(hdr, "W:") {
		return nil, fmt.Errorf("unexpected winners header: %s", hdr)
	}

	countStr := strings.TrimPrefix(hdr, "W:")
	count, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil {
		return nil, fmt.Errorf("invalid winners count in header %q: %v", hdr, err)
	}

	if count == 0 {
		// Body debe ser "N"
		return []string{}, nil
	}

	// Body = "dni1~dni2~...~dnik"
	winners := strings.Split(strings.TrimSpace(body), c.config.MessageProtocol.BatchSeparator)
	// Si querés ser más papista que el papa, podés chequear mismatch:
	// if len(winners) != count { log.Warningf("...") }
	return winners, nil
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

func readTwoLines(r *bufio.Reader, delim byte) (string, string, error) {
	header, err := r.ReadString(delim)
	if err != nil {
		return "", "", err
	}
	body, err := r.ReadString(delim)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(header), strings.TrimSpace(body), nil
}

// writeAll writes all bytes from p to conn, handling partial writes
func writeAll(conn net.Conn, p []byte) error {
	total := 0
	for total < len(p) {
		n, err := conn.Write(p[total:])
		total += n
		if err != nil {
			return fmt.Errorf("writeAll: wrote=%d/%d: %w", total, len(p), err)
		}
	}
	return nil
}
