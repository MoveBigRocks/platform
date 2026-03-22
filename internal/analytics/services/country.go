package analyticsservices

import analyticsdomain "github.com/movebigrocks/platform/internal/analytics/domain"

func CountryFromLanguage(acceptLang string) string {
	return analyticsdomain.CountryFromLanguage(acceptLang)
}
