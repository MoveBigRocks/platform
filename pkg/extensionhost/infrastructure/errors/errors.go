package apierrors

import internalerrors "github.com/movebigrocks/platform/internal/infrastructure/errors"

type ErrorType = internalerrors.ErrorType
type ValidationError = internalerrors.ValidationError

const (
	ErrorTypeValidation    = internalerrors.ErrorTypeValidation
	ErrorTypeAuthorization = internalerrors.ErrorTypeAuthorization
)

var New = internalerrors.New
var Wrap = internalerrors.Wrap
var NewValidationError = internalerrors.NewValidationError
var NewValidationErrors = internalerrors.NewValidationErrors
var DatabaseError = internalerrors.DatabaseError
var NotFoundError = internalerrors.NotFoundError
