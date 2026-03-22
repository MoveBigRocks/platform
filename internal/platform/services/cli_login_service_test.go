package platformservices

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLILoginService_StartAuthorizePoll(t *testing.T) {
	t.Parallel()

	service := NewCLILoginService(WithCLILoginTTL(5 * time.Minute))
	t.Cleanup(service.Close)

	start, err := service.Start()
	require.NoError(t, err)
	require.NotEmpty(t, start.RequestID)
	require.NotEmpty(t, start.PollToken)

	pending, err := service.Poll(start.PollToken)
	require.NoError(t, err)
	assert.Equal(t, CLILoginStatusPending, pending.Status)

	require.NoError(t, service.Authorize(start.RequestID, "user_123", "session_abc"))

	ready, err := service.Poll(start.PollToken)
	require.NoError(t, err)
	assert.Equal(t, CLILoginStatusReady, ready.Status)
	assert.Equal(t, "user_123", ready.UserID)
	assert.Equal(t, "session_abc", ready.SessionToken)

	_, err = service.Poll(start.PollToken)
	require.ErrorIs(t, err, ErrCLILoginRequestNotFound)
}

func TestCLILoginService_PollExpiredRequest(t *testing.T) {
	t.Parallel()

	service := NewCLILoginService(WithCLILoginTTL(10 * time.Millisecond))
	t.Cleanup(service.Close)

	start, err := service.Start()
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)

	_, err = service.Poll(start.PollToken)
	require.ErrorIs(t, err, ErrCLILoginExpired)
}

func TestCLILoginService_AuthorizeUnknownRequest(t *testing.T) {
	t.Parallel()

	service := NewCLILoginService()
	t.Cleanup(service.Close)

	err := service.Authorize("missing", "user_123", "session_abc")
	require.ErrorIs(t, err, ErrCLILoginRequestNotFound)
}
