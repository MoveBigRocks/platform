package outbox

import internaloutbox "github.com/movebigrocks/platform/internal/infrastructure/outbox"

type Service = internaloutbox.Service

var NewService = internaloutbox.NewService
var NewServiceWithConfig = internaloutbox.NewServiceWithConfig
