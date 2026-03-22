package antivirus

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

var (
	// ErrMalwareDetected indicates the file contains malware
	ErrMalwareDetected = errors.New("malware detected")

	// ErrScanFailed indicates the scan could not be completed
	ErrScanFailed = errors.New("scan failed")

	// ErrConnectionFailed indicates connection to ClamAV failed
	ErrConnectionFailed = errors.New("connection to antivirus service failed")
)

// ScanResult contains the result of a virus scan
type ScanResult struct {
	Clean      bool      `json:"clean"`
	VirusName  string    `json:"virus_name,omitempty"`
	Message    string    `json:"message"`
	ScannedAt  time.Time `json:"scanned_at"`
	ScanTimeMs int64     `json:"scan_time_ms"`
}

// Scanner defines the interface for antivirus scanning
type Scanner interface {
	// Scan scans a file/reader for viruses
	Scan(ctx context.Context, reader io.Reader) (*ScanResult, error)

	// Ping checks if the antivirus service is available
	Ping(ctx context.Context) error
}

// ClamAVScanner implements Scanner using ClamAV daemon
type ClamAVScanner struct {
	addr    string        // ClamAV daemon address (host:port)
	timeout time.Duration // Connection timeout
}

// ClamAVConfig holds ClamAV configuration
type ClamAVConfig struct {
	Addr    string        // Default: "localhost:3310"
	Timeout time.Duration // Default: 30s
}

// NewClamAVScanner creates a new ClamAV scanner
func NewClamAVScanner(cfg ClamAVConfig) *ClamAVScanner {
	if cfg.Addr == "" {
		cfg.Addr = "localhost:3310"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &ClamAVScanner{
		addr:    cfg.Addr,
		timeout: cfg.Timeout,
	}
}

// Scan scans the provided reader for viruses using ClamAV's INSTREAM command
func (s *ClamAVScanner) Scan(ctx context.Context, reader io.Reader) (*ScanResult, error) {
	start := time.Now()

	// Connect to ClamAV daemon
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer conn.Close()

	// Set deadline based on context or timeout
	deadline := time.Now().Add(s.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	// Best-effort deadline, ignore error
	_ = conn.SetDeadline(deadline) //nolint:errcheck // best-effort deadline

	// Send INSTREAM command (zINSTREAM\0 for null-terminated)
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Stream file content in chunks
	// ClamAV INSTREAM format: [4-byte big-endian chunk size][chunk data]...
	// End with 4-byte zero to indicate end of stream
	buf := make([]byte, 2048)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			// Write chunk size (big endian uint32)
			sizeBytes := make([]byte, 4)
			binary.BigEndian.PutUint32(sizeBytes, uint32(n))
			if _, err := conn.Write(sizeBytes); err != nil {
				return nil, fmt.Errorf("failed to write chunk size: %w", err)
			}

			// Write chunk data
			if _, err := conn.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("failed to write chunk data: %w", err)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}
	}

	// Send end-of-stream marker (4 zero bytes)
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return nil, fmt.Errorf("failed to send end marker: %w", err)
	}

	// Read response using a single reader to avoid losing buffered data
	// Some ClamAV versions null-terminate (\x00), others use newline (\n)
	connReader := bufio.NewReader(conn)
	response, err := connReader.ReadString('\x00')
	if err != nil && err != io.EOF {
		// First read failed - but the reader may have consumed partial data.
		// Try reading until newline on the SAME reader to preserve any buffered bytes.
		response, err = connReader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}
	}

	// Parse response
	response = strings.TrimRight(response, "\x00\n\r")
	elapsed := time.Since(start)

	result := &ScanResult{
		ScannedAt:  time.Now(),
		ScanTimeMs: elapsed.Milliseconds(),
	}

	// Response format: "stream: OK" or "stream: VirusName FOUND"
	switch {
	case strings.HasSuffix(response, "OK"):
		result.Clean = true
		result.Message = "No threats detected"
	case strings.Contains(response, "FOUND"):
		result.Clean = false
		// Extract virus name from response like "stream: Win.Test.EICAR_HDB-1 FOUND"
		parts := strings.Split(response, ":")
		if len(parts) > 1 {
			virusPart := strings.TrimSpace(parts[len(parts)-1])
			virusPart = strings.TrimSuffix(virusPart, " FOUND")
			result.VirusName = virusPart
		}
		result.Message = fmt.Sprintf("Threat detected: %s", result.VirusName)
	case strings.Contains(response, "ERROR"):
		return nil, fmt.Errorf("%w: %s", ErrScanFailed, response)
	default:
		// Unknown response
		return nil, fmt.Errorf("%w: unexpected response: %s", ErrScanFailed, response)
	}

	return result, nil
}

// Ping checks if ClamAV daemon is available
func (s *ClamAVScanner) Ping(ctx context.Context) error {
	conn, err := s.connect(ctx)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionFailed, err)
	}
	defer conn.Close()

	// Best-effort deadline for ping, ignore error
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck // best-effort deadline

	// Send PING command
	if _, err := conn.Write([]byte("zPING\x00")); err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}

	// Read response
	response, err := bufio.NewReader(conn).ReadString('\x00')
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read ping response: %w", err)
	}

	response = strings.TrimRight(response, "\x00\n\r")
	if response != "PONG" {
		return fmt.Errorf("unexpected ping response: %s", response)
	}

	return nil
}

// connect establishes a connection to ClamAV daemon
func (s *ClamAVScanner) connect(ctx context.Context) (net.Conn, error) {
	var d net.Dialer
	d.Timeout = s.timeout

	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// MockScanner implements Scanner for testing
type MockScanner struct {
	AlwaysClean bool
	VirusName   string
	ShouldError bool
	ErrorMsg    string
}

// Scan returns a mock scan result
func (m *MockScanner) Scan(ctx context.Context, reader io.Reader) (*ScanResult, error) {
	// Consume the reader
	_, _ = io.Copy(io.Discard, reader)

	if m.ShouldError {
		return nil, fmt.Errorf("%w: %s", ErrScanFailed, m.ErrorMsg)
	}

	if m.AlwaysClean {
		return &ScanResult{
			Clean:      true,
			Message:    "No threats detected (mock)",
			ScannedAt:  time.Now(),
			ScanTimeMs: 1,
		}, nil
	}

	return &ScanResult{
		Clean:      false,
		VirusName:  m.VirusName,
		Message:    fmt.Sprintf("Threat detected: %s (mock)", m.VirusName),
		ScannedAt:  time.Now(),
		ScanTimeMs: 1,
	}, nil
}

// Ping always succeeds for mock
func (m *MockScanner) Ping(ctx context.Context) error {
	if m.ShouldError {
		return ErrConnectionFailed
	}
	return nil
}

// NewMockScanner creates a mock scanner for testing
func NewMockScanner(alwaysClean bool) *MockScanner {
	return &MockScanner{
		AlwaysClean: alwaysClean,
		VirusName:   "Test.Virus.Mock",
	}
}
