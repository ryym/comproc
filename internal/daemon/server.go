package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ryym/comproc/internal/protocol"
)

const gracefulTimeout = 10 * time.Second

// Server handles JSON-RPC requests from clients.
type Server struct {
	daemon     *Daemon
	socketPath string
	listener   net.Listener
	mu         sync.Mutex
	conns      map[net.Conn]bool
}

// NewServer creates a new RPC server.
func NewServer(d *Daemon, socketPath string) *Server {
	return &Server{
		daemon:     d,
		socketPath: socketPath,
		conns:      make(map[net.Conn]bool),
	}
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	// Remove existing socket file
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Accept connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}

			s.mu.Lock()
			s.conns[conn] = true
			s.mu.Unlock()

			go s.handleConnection(ctx, conn)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()

	// Close listener and all connections
	listener.Close()
	s.mu.Lock()
	for conn := range s.conns {
		conn.Close()
	}
	s.mu.Unlock()

	// Clean up socket file
	os.Remove(s.socketPath)

	return nil
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() {
		conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	reader := bufio.NewReader(conn)
	encoder := json.NewEncoder(conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read a line (JSON-RPC request)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var req protocol.Request
		if err := json.Unmarshal(line, &req); err != nil {
			resp := protocol.NewErrorResponse(protocol.ParseError, "invalid JSON", nil)
			encoder.Encode(resp)
			continue
		}

		resp := s.handleRequest(ctx, conn, &req)
		if resp != nil {
			encoder.Encode(resp)
		}
	}
}

// handleRequest processes a single RPC request.
func (s *Server) handleRequest(ctx context.Context, conn net.Conn, req *protocol.Request) *protocol.Response {
	switch req.Method {
	case protocol.MethodUp:
		return s.handleUp(req)
	case protocol.MethodDown:
		return s.handleDown(req)
	case protocol.MethodStatus:
		return s.handleStatus(req)
	case protocol.MethodRestart:
		return s.handleRestart(req)
	case protocol.MethodLogs:
		return s.handleLogs(ctx, conn, req)
	default:
		return protocol.NewErrorResponse(protocol.MethodNotFound, "method not found", req.ID)
	}
}

func (s *Server) handleUp(req *protocol.Request) *protocol.Response {
	var params protocol.UpParams
	if err := req.ParseParams(&params); err != nil {
		return protocol.NewErrorResponse(protocol.InvalidParams, err.Error(), req.ID)
	}

	started, failed := s.daemon.StartServices(params.Services)

	result := protocol.UpResult{
		Started: started,
		Failed:  failed,
	}

	resp, err := protocol.NewResponse(result, *req.ID)
	if err != nil {
		return protocol.NewErrorResponse(protocol.InternalError, err.Error(), req.ID)
	}
	return resp
}

func (s *Server) handleDown(req *protocol.Request) *protocol.Response {
	var params protocol.DownParams
	if err := req.ParseParams(&params); err != nil {
		return protocol.NewErrorResponse(protocol.InvalidParams, err.Error(), req.ID)
	}

	stopped := s.daemon.StopServices(params.Services)

	result := protocol.DownResult{
		Stopped: stopped,
	}

	resp, err := protocol.NewResponse(result, *req.ID)
	if err != nil {
		return protocol.NewErrorResponse(protocol.InternalError, err.Error(), req.ID)
	}
	return resp
}

func (s *Server) handleStatus(req *protocol.Request) *protocol.Response {
	statuses := s.daemon.GetStatus()

	var protoStatuses []protocol.ServiceStatus
	for _, st := range statuses {
		protoStatuses = append(protoStatuses, protocol.ServiceStatus{
			Name:      st.Name,
			State:     st.State,
			PID:       st.PID,
			Restarts:  st.Restarts,
			StartedAt: st.StartedAt,
			ExitCode:  st.ExitCode,
		})
	}

	result := protocol.StatusResult{
		Services: protoStatuses,
	}

	resp, err := protocol.NewResponse(result, *req.ID)
	if err != nil {
		return protocol.NewErrorResponse(protocol.InternalError, err.Error(), req.ID)
	}
	return resp
}

func (s *Server) handleRestart(req *protocol.Request) *protocol.Response {
	var params protocol.RestartParams
	if err := req.ParseParams(&params); err != nil {
		return protocol.NewErrorResponse(protocol.InvalidParams, err.Error(), req.ID)
	}

	restarted, failed := s.daemon.RestartServices(params.Services)

	result := protocol.RestartResult{
		Restarted: restarted,
		Failed:    failed,
	}

	resp, err := protocol.NewResponse(result, *req.ID)
	if err != nil {
		return protocol.NewErrorResponse(protocol.InternalError, err.Error(), req.ID)
	}
	return resp
}

func (s *Server) handleLogs(ctx context.Context, conn net.Conn, req *protocol.Request) *protocol.Response {
	var params protocol.LogsParams
	if err := req.ParseParams(&params); err != nil {
		return protocol.NewErrorResponse(protocol.InvalidParams, err.Error(), req.ID)
	}

	// Get recent logs
	lines := params.Lines
	if lines <= 0 {
		lines = 100
	}
	logs := s.daemon.GetLogs(params.Services, lines)

	// Send initial response
	result := struct {
		Lines []protocol.LogEntry `json:"lines"`
	}{
		Lines: make([]protocol.LogEntry, 0, len(logs)),
	}
	for _, l := range logs {
		result.Lines = append(result.Lines, protocol.LogEntry{
			Service:   l.Service,
			Line:      l.Line,
			Timestamp: l.Timestamp.Format(time.RFC3339),
			Stream:    l.Stream,
		})
	}

	resp, err := protocol.NewResponse(result, *req.ID)
	if err != nil {
		return protocol.NewErrorResponse(protocol.InternalError, err.Error(), req.ID)
	}

	// If follow mode, start streaming
	if params.Follow {
		encoder := json.NewEncoder(conn)

		// Send initial response first
		encoder.Encode(resp)

		// Subscribe to log updates
		ch := s.daemon.SubscribeLogs(params.Services)
		defer s.daemon.UnsubscribeLogs(ch)

		for {
			select {
			case <-ctx.Done():
				return nil
			case line, ok := <-ch:
				if !ok {
					return nil
				}
				entry := protocol.LogEntry{
					Service:   line.Service,
					Line:      line.Line,
					Timestamp: line.Timestamp.Format(time.RFC3339),
					Stream:    line.Stream,
				}
				notification, _ := protocol.NewNotification(protocol.MethodLog, entry)
				if err := encoder.Encode(notification); err != nil {
					return nil
				}
			}
		}
	}

	return resp
}
