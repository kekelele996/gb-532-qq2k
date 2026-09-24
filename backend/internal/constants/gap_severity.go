package constants

import "fmt"

type GapSeverity string
type GapState string

const (
	SeverityMinor    GapSeverity = "minor"
	SeverityMajor    GapSeverity = "major"
	SeverityCritical GapSeverity = "critical"

	GapDetected      GapState = "detected"
	GapReviewed      GapState = "reviewed"
	GapAccepted      GapState = "accepted"
	GapFalsePositive GapState = "false_positive"
	GapRetesting     GapState = "retesting"
	GapResurveyed    GapState = "resurveyed"
	GapClosed        GapState = "closed"
)

// gapTransitions 仅描述复核人手工迁移；accepted -> retesting 与
// retesting -> accepted/resurveyed 由补测执行单在事务内驱动，不允许手工跳转。
var gapTransitions = map[GapState]map[GapState]struct{}{
	GapDetected:      {GapReviewed: {}},
	GapReviewed:      {GapAccepted: {}, GapFalsePositive: {}},
	GapAccepted:      {},
	GapFalsePositive: {GapClosed: {}},
	GapRetesting:     {},
	GapResurveyed:    {GapClosed: {}},
	GapClosed:        {},
}

func (s GapState) Valid() bool {
	_, ok := gapTransitions[s]
	return ok
}

func (s GapState) CanTransition(target GapState) bool {
	allowed, ok := gapTransitions[s]
	if !ok {
		return false
	}
	_, ok = allowed[target]
	return ok
}

func ParseGapState(value string) (GapState, error) {
	state := GapState(value)
	if !state.Valid() {
		return "", fmt.Errorf("unknown gap state %q", value)
	}
	return state, nil
}

func SeverityForRatio(ratio float64) GapSeverity {
	switch {
	case ratio >= 0.12:
		return SeverityCritical
	case ratio >= 0.05:
		return SeverityMajor
	default:
		return SeverityMinor
	}
}
