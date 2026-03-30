package container

import internalcontainer "github.com/movebigrocks/platform/internal/infrastructure/container"

type PlatformContainer = internalcontainer.PlatformContainer
type ServiceContainer = internalcontainer.ServiceContainer
type ServiceContainerDeps = internalcontainer.ServiceContainerDeps

var NewPlatformContainer = internalcontainer.NewPlatformContainer
var NewServiceContainer = internalcontainer.NewServiceContainer
