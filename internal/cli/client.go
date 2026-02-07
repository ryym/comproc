// Package cli implements the comproc CLI commands.
package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ryym/comproc/internal/protocol"
)

// Client communicates with the comproc daemon.
type Client struct {
	socketPath string
	conn       net.Conn
	reader     *bufio.Reader
	encoder    *json.Encoder
	nextID     atomic.Int32
}

// NewClient creates a new client.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
	}
}

// Connect connects to the daemon.
func (c *Client) Connect() error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.encoder = json.NewEncoder(conn)
	return nil
}

// Close closes the connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Call sends a request and waits for a response.
func (c *Client) Call(method string, params any) (*protocol.Response, error) {
	id := int(c.nextID.Add(1))
	req, err := protocol.NewRequest(method, params, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if err := c.encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp protocol.Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	return &resp, nil
}

// ReadNotification reads a notification from the connection.
func (c *Client) ReadNotification() (*protocol.Request, error) {
	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	var req protocol.Request
	if err := json.Unmarshal(line, &req); err != nil {
		return nil, fmt.Errorf("failed to parse notification: %w", err)
	}

	return &req, nil
}

// Up starts services.
func (c *Client) Up(services []string) (*protocol.UpResult, error) {
	params := protocol.UpParams{Services: services}
	resp, err := c.Call(protocol.MethodUp, params)
	if err != nil {
		return nil, err
	}

	var result protocol.UpResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Shutdown shuts down the daemon, stopping all services.
func (c *Client) Shutdown() (*protocol.ShutdownResult, error) {
	resp, err := c.Call(protocol.MethodShutdown, nil)
	if err != nil {
		return nil, err
	}

	var result protocol.ShutdownResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Down stops services.
func (c *Client) Down(services []string) (*protocol.DownResult, error) {
	params := protocol.DownParams{Services: services}
	resp, err := c.Call(protocol.MethodDown, params)
	if err != nil {
		return nil, err
	}

	var result protocol.DownResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Status gets service statuses.
func (c *Client) Status() (*protocol.StatusResult, error) {
	resp, err := c.Call(protocol.MethodStatus, nil)
	if err != nil {
		return nil, err
	}

	var result protocol.StatusResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Restart restarts services.
func (c *Client) Restart(services []string) (*protocol.RestartResult, error) {
	params := protocol.RestartParams{Services: services}
	resp, err := c.Call(protocol.MethodRestart, params)
	if err != nil {
		return nil, err
	}

	var result protocol.RestartResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// LogsResult contains the initial logs response.
type LogsResult struct {
	Lines []protocol.LogEntry `json:"lines"`
}

// Logs gets service logs.
func (c *Client) Logs(services []string, lines int, follow bool) (*LogsResult, error) {
	params := protocol.LogsParams{
		Services: services,
		Lines:    lines,
		Follow:   follow,
	}
	resp, err := c.Call(protocol.MethodLogs, params)
	if err != nil {
		return nil, err
	}

	var result LogsResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Attach attaches to a service's stdin/stdout.
func (c *Client) Attach(service string) (*protocol.AttachResult, error) {
	params := protocol.AttachParams{Service: service}
	resp, err := c.Call(protocol.MethodAttach, params)
	if err != nil {
		return nil, err
	}

	var result protocol.AttachResult
	if err := resp.ParseResult(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SendStdin sends stdin data to the daemon as a notification.
func (c *Client) SendStdin(data string) error {
	notification, err := protocol.NewNotification(protocol.MethodStdin, protocol.StdinData{Data: data})
	if err != nil {
		return err
	}
	return c.encoder.Encode(notification)
}
