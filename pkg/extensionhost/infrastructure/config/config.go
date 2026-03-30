package config

import internalconfig "github.com/movebigrocks/platform/internal/infrastructure/config"

type Config = internalconfig.Config
type DatabaseConfig = internalconfig.DatabaseConfig
type OutboxConfig = internalconfig.OutboxConfig
type ErrorProcessingConfig = internalconfig.ErrorProcessingConfig

var Load = internalconfig.Load
