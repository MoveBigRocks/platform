package geoip

import internalgeoip "github.com/movebigrocks/platform/internal/shared/geoip"

type Service = internalgeoip.Service
type GeoLocation = internalgeoip.GeoLocation

var NewNoopService = internalgeoip.NewNoopService
var NewMaxMindService = internalgeoip.NewMaxMindService
