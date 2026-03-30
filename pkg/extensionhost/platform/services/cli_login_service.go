package platformservices

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/movebigrocks/platform/pkg/logger"
)

var (
	ErrCLILoginRequestNotFound = errors.New("cli login request not found")
	ErrCLILoginExpired         = errors.New("cli login request expired")
)

type CLILoginStatus string

const (
	CLILoginStatusPending  CLILoginStatus = "pending"
	CLILoginStatusReady    CLILoginStatus = "ready"
	CLILoginStatusConsumed CLILoginStatus = "consumed"
)

type CLILoginStart struct {
	RequestID string
	PollToken string
	ExpiresAt time.Time
}

type CLILoginPollResult struct {
	Status       CLILoginStatus
	UserID       string
	SessionToken string
	ExpiresAt    time.Time
}

type cliLoginRequest struct {
	RequestID    string
	PollToken    string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	AuthorizedAt *time.Time
	ConsumedAt   *time.Time
	UserID       string
	SessionToken string
}

// CLILoginService manages one-time browser login exchanges for the CLI.
type CLILoginService struct {
	mu        sync.RWMutex
	ttl       time.Duration
	requests  map[string]*cliLoginRequest
	pollIndex map[string]string
	stop      chan struct{}
	logger    *logger.Logger
}

type CLILoginServiceOption func(*CLILoginService)

func WithCLILoginTTL(ttl time.Duration) CLILoginServiceOption {
	return func(s *CLILoginService) {
		if ttl > 0 {
			s.ttl = ttl
		}
	}
}

func WithCLILoginLogger(log *logger.Logger) CLILoginServiceOption {
	return func(s *CLILoginService) {
		if log != nil {
			s.logger = log
		}
	}
}

func NewCLILoginService(opts ...CLILoginServiceOption) *CLILoginService {
	svc := &CLILoginService{
		ttl:       10 * time.Minute,
		requests:  make(map[string]*cliLoginRequest),
		pollIndex: make(map[string]string),
		stop:      make(chan struct{}),
		logger:    logger.NewNop(),
	}
	for _, opt := range opts {
		opt(svc)
	}
	go svc.cleanupLoop()
	return svc
}

func (s *CLILoginService) Close() {
	select {
	case <-s.stop:
		return
	default:
		close(s.stop)
	}
}

func (s *CLILoginService) Start() (*CLILoginStart, error) {
	requestID, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	pollToken, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	request := &cliLoginRequest{
		RequestID: requestID,
		PollToken: pollToken,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.mu.Lock()
	s.requests[requestID] = request
	s.pollIndex[pollToken] = requestID
	s.mu.Unlock()

	return &CLILoginStart{
		RequestID: requestID,
		PollToken: pollToken,
		ExpiresAt: request.ExpiresAt,
	}, nil
}

func (s *CLILoginService) Authorize(requestID, userID, sessionToken string) error {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	request, ok := s.requests[requestID]
	if !ok {
		return ErrCLILoginRequestNotFound
	}
	if now.After(request.ExpiresAt) {
		s.deleteLocked(request)
		return ErrCLILoginExpired
	}
	if request.ConsumedAt != nil {
		return ErrCLILoginRequestNotFound
	}

	request.UserID = userID
	request.SessionToken = sessionToken
	request.AuthorizedAt = &now
	return nil
}

func (s *CLILoginService) Poll(pollToken string) (*CLILoginPollResult, error) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	requestID, ok := s.pollIndex[pollToken]
	if !ok {
		return nil, ErrCLILoginRequestNotFound
	}
	request, ok := s.requests[requestID]
	if !ok {
		delete(s.pollIndex, pollToken)
		return nil, ErrCLILoginRequestNotFound
	}
	if now.After(request.ExpiresAt) {
		s.deleteLocked(request)
		return nil, ErrCLILoginExpired
	}
	if request.ConsumedAt != nil {
		return nil, ErrCLILoginRequestNotFound
	}
	if request.AuthorizedAt == nil {
		return &CLILoginPollResult{
			Status:    CLILoginStatusPending,
			ExpiresAt: request.ExpiresAt,
		}, nil
	}

	request.ConsumedAt = &now
	result := &CLILoginPollResult{
		Status:       CLILoginStatusReady,
		UserID:       request.UserID,
		SessionToken: request.SessionToken,
		ExpiresAt:    request.ExpiresAt,
	}
	s.deleteLocked(request)
	return result, nil
}

func (s *CLILoginService) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.cleanupExpired()
		}
	}
}

func (s *CLILoginService) cleanupExpired() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, request := range s.requests {
		if now.After(request.ExpiresAt) || request.ConsumedAt != nil {
			s.deleteLocked(request)
		}
	}
}

func (s *CLILoginService) deleteLocked(request *cliLoginRequest) {
	delete(s.requests, request.RequestID)
	delete(s.pollIndex, request.PollToken)
}

func randomHex(byteLength int) (string, error) {
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
