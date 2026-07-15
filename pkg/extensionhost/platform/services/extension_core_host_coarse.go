package platformservices

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	serviceapp "github.com/movebigrocks/platform/pkg/extensionhost/service/services"
)

const ingestApplicationOperation = "ingestApplication"

// IngestApplication creates a core contact and case for an application and links
// its attachments, all in one workspace transaction, and records the result
// under the caller's idempotency key. A repeat call with the same key returns
// the stored identifiers without creating new rows, which is what lets the
// extension retry after a partial failure without duplicating core data.
func (s *ExtensionCoreHostService) IngestApplication(ctx context.Context, extensionID string, input runtimehost.IngestApplicationInput) (*runtimehost.IngestApplicationResult, error) {
	if s == nil || s.extensions == nil || s.cases == nil || s.contacts == nil || s.tenant == nil {
		return nil, fmt.Errorf("core host services are not configured")
	}
	extension, err := s.resolveExtension(ctx, extensionID, "case:write")
	if err != nil {
		return nil, err
	}
	// A coarse ingest writes contacts, cases, and attachment links; require the
	// extension to hold each of those permissions, not just case:write.
	for _, permission := range []string{"contact:write", "attachment:write"} {
		if !manifestHasPermission(extension.Manifest, permission) {
			return nil, ErrExtensionHostForbidden
		}
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if key == "" {
		return nil, fmt.Errorf("idempotencyKey is required")
	}
	if strings.TrimSpace(input.Contact.Email) == "" {
		return nil, fmt.Errorf("contact email is required")
	}
	if strings.TrimSpace(input.Case.Subject) == "" {
		return nil, fmt.Errorf("case subject is required")
	}

	var result *runtimehost.IngestApplicationResult
	err = s.tenant.WithTransaction(ctx, func(txCtx context.Context) error {
		if err := s.tenant.SetTenantContext(txCtx, extension.WorkspaceID); err != nil {
			return err
		}

		stored, found, ferr := s.tenant.GetHostOperationResult(txCtx, extension.WorkspaceID, extension.ID, ingestApplicationOperation, key)
		if ferr != nil {
			return ferr
		}
		if found {
			var prior runtimehost.IngestApplicationResult
			if uerr := json.Unmarshal(stored, &prior); uerr != nil {
				return uerr
			}
			result = &prior
			return nil
		}

		contact, cerr := s.contacts.CreateContact(txCtx, CreateContactParams{
			WorkspaceID: extension.WorkspaceID,
			Email:       strings.TrimSpace(input.Contact.Email),
			Name:        strings.TrimSpace(input.Contact.Name),
			Phone:       strings.TrimSpace(input.Contact.Phone),
			Company:     strings.TrimSpace(input.Contact.Company),
			Source:      strings.TrimSpace(input.Contact.Source),
			Metadata:    input.Contact.Metadata,
		})
		if cerr != nil {
			return cerr
		}

		caseObj, kerr := s.cases.CreateCase(txCtx, serviceapp.CreateCaseParams{
			WorkspaceID:  extension.WorkspaceID,
			Subject:      strings.TrimSpace(input.Case.Subject),
			Description:  input.Case.Description,
			Priority:     servicedomain.CasePriority(strings.TrimSpace(input.Case.Priority)),
			Channel:      servicedomain.CaseChannel(strings.TrimSpace(input.Case.Channel)),
			Category:     strings.TrimSpace(input.Case.Category),
			QueueID:      strings.TrimSpace(input.Case.QueueID),
			ContactID:    contact.ID,
			ContactName:  strings.TrimSpace(input.Contact.Name),
			ContactEmail: strings.TrimSpace(input.Contact.Email),
			Tags:         input.Case.Tags,
			CustomFields: customFieldsFromMap(input.Case.CustomFields),
		})
		if kerr != nil {
			return kerr
		}

		if len(input.AttachmentIDs) > 0 && s.attachmentStore != nil {
			if lerr := s.attachmentStore.LinkAttachmentsToCase(txCtx, extension.WorkspaceID, caseObj.ID, input.AttachmentIDs); lerr != nil {
				return lerr
			}
		}

		result = &runtimehost.IngestApplicationResult{ContactID: contact.ID, CaseID: caseObj.ID}
		payload, merr := json.Marshal(result)
		if merr != nil {
			return merr
		}
		return s.tenant.PutHostOperationResult(txCtx, extension.WorkspaceID, extension.ID, ingestApplicationOperation, key, payload)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
