package constants

const (
	MeasureTypeShielding     = "shielding"
	MeasureTypeDistance      = "distance"
	MeasureTypeRotation      = "rotation"
	MeasureTypeAuthorization = "authorization"
)

var MeasureTypes = []string{
	MeasureTypeShielding,
	MeasureTypeDistance,
	MeasureTypeRotation,
	MeasureTypeAuthorization,
}

func IsMeasureType(value string) bool {
	for _, candidate := range MeasureTypes {
		if value == candidate {
			return true
		}
	}
	return false
}

// MeasureFormulaVersion 冻结进每个预算情景证据的折减公式版本。
const MeasureFormulaVersion = "MEASURE-2026.1"
