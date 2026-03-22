package dtos

import (
	"time"

	servicedomain "github.com/movebigrocks/platform/internal/service/domain"
)

type CaseResponse struct {
	ID             string     `json:"id"`
	HumanID        string     `json:"human_id"`
	Subject        string     `json:"subject"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	Priority       string     `json:"priority"`
	ContactID      string     `json:"contact_id"`
	AssignedToID   string     `json:"assigned_to_id"`
	TeamID         string     `json:"team_id"`
	Tags           []string   `json:"tags"`
	Resolution     string     `json:"resolution"`
	ResolutionNote string     `json:"resolution_note"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ToCaseResponse(c *servicedomain.Case) CaseResponse {
	return CaseResponse{
		ID:             c.ID,
		HumanID:        c.HumanID,
		Subject:        c.Subject,
		Description:    c.Description,
		Status:         string(c.Status),
		Priority:       string(c.Priority),
		ContactID:      c.ContactID,
		AssignedToID:   c.AssignedToID,
		TeamID:         c.TeamID,
		Tags:           c.Tags,
		Resolution:     "", // c.Resolution not in Case struct, handled via status
		ResolutionNote: "", // c.ResolutionNote not in Case struct
		ResolvedAt:     c.ResolvedAt,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

type CreateCaseRequest struct {
	Subject      string   `json:"subject" binding:"required"`
	Description  string   `json:"description" binding:"required"`
	Priority     string   `json:"priority"`
	ContactID    string   `json:"contact_id" binding:"required"`
	AssignedToID string   `json:"assigned_to_id"`
	TeamID       string   `json:"team_id"`
	Tags         []string `json:"tags"`
}
