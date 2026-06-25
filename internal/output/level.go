package output

import "github.com/liciomatos/pgloop/internal/lockmapper"

func riskToLevel(risk lockmapper.RiskLevel) string {
	switch risk {
	case lockmapper.RiskCritical:
		return "error"
	case lockmapper.RiskWarn:
		return "warning"
	default:
		return "notice"
	}
}
