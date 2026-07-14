package ffuf

// MarkovFeedback interface for Markov Chain based feedback mechanism
type MarkovFeedback interface {
	UpdateWithResponse(resp Response, isMatch bool)
	UpdateWithMatchedInput(input map[string][]byte)
	GetNextInput(baseInputProvider InputProvider) (map[string][]byte, bool)
	AnalyzeResponsePatterns() map[string]interface{}
	SetVerbose(verbose bool)
	GetVerbose() bool
	PrintMarkovInfo()
}