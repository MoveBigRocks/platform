package analyticsservices

import analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"

func ClassifySource(utmSource, referrer, propertyDomain string) string {
	return analyticsdomain.ClassifySource(utmSource, referrer, propertyDomain)
}
