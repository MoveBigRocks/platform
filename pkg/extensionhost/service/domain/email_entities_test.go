package servicedomain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEmailBlacklistLifecycle(t *testing.T) {
	pattern := NewEmailBlacklist("ws_1", "pattern", ".*@spam\\.example", "abuse", "user_1")
	require.NoError(t, pattern.ValidatePattern())
	require.True(t, pattern.IsBlocked("bad@spam.example", "spam.example"))
	require.False(t, pattern.IsBlocked("good@example.com", "example.com"))

	email := NewEmailBlacklist("ws_1", "email", "blocked@example.com", "manual", "user_1")
	require.True(t, email.IsBlocked("blocked@example.com", "example.com"))

	domain := NewEmailBlacklist("ws_1", "domain", "blocked.example", "manual", "user_1")
	require.True(t, domain.IsBlocked("user@blocked.example", "blocked.example"))

	expiredAt := time.Now().Add(-time.Hour)
	email.ExpiresAt = &expiredAt
	require.False(t, email.IsBlocked("blocked@example.com", "example.com"))
}

func TestInboundAndOutboundEmailDomainBehavior(t *testing.T) {
	inbound := NewInboundEmail("ws_1", "message_1", "from@example.com", "Hello", "Body")
	require.Equal(t, EmailProcessingStatusPending, inbound.ProcessingStatus)
	require.NotNil(t, inbound.Headers)

	before := inbound.UpdatedAt
	inbound.MarkUpdated()
	require.True(t, inbound.UpdatedAt.After(before) || inbound.UpdatedAt.Equal(before))

	outbound := NewOutboundEmail("ws_1", "sender@example.com", "Subject", "Body")
	outbound.ToEmails = []string{"recipient@example.com"}
	require.NoError(t, outbound.Validate())

	outbound.MarkOpened()
	require.NotNil(t, outbound.OpenedAt)
	require.Equal(t, 1, outbound.OpenCount)

	outbound.MarkClicked()
	require.NotNil(t, outbound.LastClickAt)
	require.Equal(t, 1, outbound.ClickCount)

	outbound.ToEmails = nil
	require.EqualError(t, outbound.Validate(), "at least one recipient is required")
}

func TestEmailThreadBehavior(t *testing.T) {
	thread := &EmailThread{
		UnreadCount:  2,
		Participants: []ThreadParticipant{},
	}

	thread.AddEmail("email_1", "message_1", time.Now(), true, 2)
	require.Equal(t, 1, thread.EmailCount)
	require.Equal(t, 3, thread.UnreadCount)
	require.True(t, thread.HasAttachments)
	require.Equal(t, 2, thread.AttachmentCount)

	thread.AddParticipant(ThreadParticipant{Email: "user@example.com", LastSeenAt: time.Now(), EmailCount: 1})
	thread.AddParticipant(ThreadParticipant{Email: "user@example.com", LastSeenAt: time.Now(), EmailCount: 1})
	require.Len(t, thread.Participants, 1)
	require.Equal(t, 2, thread.Participants[0].EmailCount)

	thread.MarkAsRead(10)
	require.Equal(t, 0, thread.UnreadCount)

	thread.SetImportant(true)
	require.True(t, thread.IsImportant)

	thread.Archive()
	require.True(t, thread.IsArchived)
	require.Equal(t, ThreadStatusArchived, thread.Status)

	thread.Close()
	require.Equal(t, ThreadStatusClosed, thread.Status)

	thread.Merge("thread_2")
	require.Equal(t, ThreadStatusMerged, thread.Status)
	require.Equal(t, "thread_2", thread.MergedIntoID)

	thread.AddChildThread("child_1")
	thread.AddChildThread("child_1")
	require.Equal(t, []string{"child_1"}, thread.ChildThreadIDs)

	thread.UpdateSentiment(-0.5)
	require.Equal(t, -0.5, thread.SentimentScore)

	thread.Watch()
	thread.Mute()
	require.True(t, thread.IsWatched)
	require.True(t, thread.IsMuted)

	thread.MarkAsSpam(0.9)
	require.True(t, thread.IsSpam)
	require.Equal(t, 0.9, thread.SpamScore)
	require.Equal(t, ThreadStatusSpam, thread.Status)

	before := thread.UpdatedAt
	thread.UpdateLastActivity()
	require.True(t, thread.UpdatedAt.After(before) || thread.UpdatedAt.Equal(before))
}
