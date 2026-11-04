package radio

func radioTickNeedsReopen(reading *RadioReading, code Code) bool {
	// A data diagnostic does not invalidate SSH. The adapter marks failed execs
	// with fixed exit/stderr reasons; transport codes retain the normal reopen path.
	if code == Partial && reading != nil && reading.Failure != nil {
		return reading.Failure.Reason == "exit" || reading.Failure.Reason == "stderr"
	}
	return code != OK || (reading != nil && reading.CommandSeconds > 3)
}
