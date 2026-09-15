package hookdeck

import "testing"

// TestTransformationRunResponse_Failed pins the success predicate against
// behaviour observed from the live API. The cases below are transcribed from
// real runs, not assumed:
//
//	addHandler("transform",(r,c)=>{return r;})                  info   request
//	addHandler("transform",(r,c)=>{console.warn("x");return r;}) warn  request
//	addHandler("transform",(r,c)=>{console.error("x");return r;}) error request
//	addHandler("transform",(r,c)=>{})                            fatal  none
//	addHandler("transform",(r,c)=>{throw new Error("boom");})     fatal  none
//
// The middle case is the one that matters: log_level is the highest severity
// the run logged, not a completion flag. An earlier version of Failed() treated
// "error" as failure, which discarded a successfully transformed request and
// exited non-zero on code that worked.
func TestTransformationRunResponse_Failed(t *testing.T) {
	request := &TransformationRunRequestInput{Headers: map[string]string{"content-type": "application/json"}}

	tests := []struct {
		name     string
		response *TransformationRunResponse
		want     bool
	}{
		{"clean run", &TransformationRunResponse{LogLevel: "info", Request: request}, false},
		{"logged a warning but returned", &TransformationRunResponse{LogLevel: "warn", Request: request}, false},
		{"logged an error but returned", &TransformationRunResponse{LogLevel: "error", Request: request}, false},
		{"handler returned nothing", &TransformationRunResponse{LogLevel: "fatal"}, true},
		{"handler threw", &TransformationRunResponse{LogLevel: "fatal"}, true},
		{"no request without a fatal level", &TransformationRunResponse{LogLevel: "info"}, true},
		{"nil response", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.Failed(); got != tt.want {
				t.Errorf("Failed() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTransformationRunResponse_ConsoleText covers the other half of #410: the
// failure reason only ever arrives in the console array, so a run that threw
// has nothing to say about itself unless this renders it.
func TestTransformationRunResponse_ConsoleText(t *testing.T) {
	tests := []struct {
		name     string
		response *TransformationRunResponse
		want     string
	}{
		{"nil response", nil, ""},
		{"no console output", &TransformationRunResponse{LogLevel: "info"}, ""},
		{
			name: "a throwing handler's error",
			response: &TransformationRunResponse{
				LogLevel: "fatal",
				Console:  []TransformationConsoleLine{{Type: "error", Message: "Error: boom"}},
			},
			want: "[error] Error: boom",
		},
		{
			name: "several lines are one per line",
			response: &TransformationRunResponse{
				LogLevel: "error",
				Console: []TransformationConsoleLine{
					{Type: "log", Message: "starting"},
					{Type: "error", Message: "bad input"},
				},
			},
			want: "[log] starting\n[error] bad input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.ConsoleText(); got != tt.want {
				t.Errorf("ConsoleText() = %q, want %q", got, tt.want)
			}
		})
	}
}
