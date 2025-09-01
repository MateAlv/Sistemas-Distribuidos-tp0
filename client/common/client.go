package common

import (
	"bufio"
	"fmt"
	"net"
	"time"
	"os"
    "os/signal"
    "syscall"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            	string
	ServerAddress 	string
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
	sigChan chan os.Signal 
	bet 			Bet
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig, bet Bet) *Client {
	client := &Client{
		config: config,
		sigChan: make(chan os.Signal, 1),
		bet: bet,
	}

	signal.Notify(client.sigChan, syscall.SIGTERM)
	return client
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

// SendBet sends the lottery bet to the server
func (c *Client) SendBet() error {
    // Create connection
    if err := c.createClientSocket(); err != nil {
        return err
    }
    defer c.conn.Close()

    // Serialize and send bet
    data := c.bet.Serialize()
    fmt.Fprintf(c.conn, "%s\n", data)

    // Read confirmation from server
    response, err := bufio.NewReader(c.conn).ReadString('\n')
    if err != nil {
        log.Errorf("action: receive_confirmation | result: fail | client_id: %v | error: %v",
            c.config.ID, err)
        return err
    }

    // Log success according to requirements
    log.Infof("action: apuesta_enviada | result: success | dni: %s | numero: %d",
        c.bet.Document, c.bet.Number)

    log.Infof("action: receive_confirmation | result: success | client_id: %v | msg: %v",
        c.config.ID, response)

    return nil
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	go func() {
        <-c.sigChan
        log.Infof("action: sigterm_received | result: success | client_id: %v", c.config.ID)
        c.GracefulShutdown()
    }()
	    			
	if !c.bet.IsValid() {
        log.Errorf("action: validate_bet | result: fail | client_id: %v", c.config.ID)
        return
    }

    // Send lottery bet (single operation, no loop)
    if err := c.SendBet(); err != nil {
        log.Errorf("action: send_bet | result: fail | client_id: %v | error: %v",
            c.config.ID, err)
        return
    }

    log.Infof("action: bet_processed | result: success | client_id: %v", c.config.ID)
}

// GracefulShutdown makes sure all resources are released properly when the client is shutting down
func (c *Client) GracefulShutdown() {
    log.Infof("action: client_shutdown | result: in_progress | client_id: %v", c.config.ID)
    
    if c.conn != nil {
        log.Infof("action: close_connection | result: success | client_id: %v", c.config.ID)
        c.conn.Close()
    }
    
    log.Infof("action: client_shutdown | result: success | client_id: %v", c.config.ID)
    os.Exit(0)
}