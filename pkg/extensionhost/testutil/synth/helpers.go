package synth

import "time"

func failScenario(result *ScenarioResult, start time.Time, err error) (*ScenarioResult, error) {
	result.Error = err
	result.Success = false
	result.Duration = time.Since(start)
	return result, nil
}

func attachmentDetail(ok bool, detail func() string) string {
	if !ok {
		return "attachment not retrieved"
	}
	return detail()
}
