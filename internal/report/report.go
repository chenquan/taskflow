package report

import (
	"encoding/json"
	"fmt"
	"io"
)

const SchemaVersion = 1

type ExitCode int

const (
	ExitOK ExitCode = iota
	ExitExecution
	ExitConfig
	ExitEnvironment
	ExitPartial
	ExitConflict
	ExitToolCompatibility
	ExitValidation
)

type Diagnostic struct {
	Code    string `json:"code"`
	Repo    string `json:"repo,omitempty"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}
type Result struct {
	SchemaVersion int          `json:"schemaVersion"`
	Command       string       `json:"command"`
	OK            bool         `json:"ok"`
	TaskID        string       `json:"taskID,omitempty"`
	Data          any          `json:"data"`
	Warnings      []Diagnostic `json:"warnings"`
	Errors        []Diagnostic `json:"errors"`
}

func New(command, taskID string) Result {
	return Result{SchemaVersion: SchemaVersion, Command: command, OK: true, TaskID: taskID, Data: map[string]any{}, Warnings: []Diagnostic{}, Errors: []Diagnostic{}}
}
func (r *Result) Fail(d Diagnostic) { r.OK = false; r.Errors = append(r.Errors, d) }
func (r *Result) Warn(d Diagnostic) { r.Warnings = append(r.Warnings, d) }
func Render(w io.Writer, r Result, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if r.OK {
		fmt.Fprintf(w, "%s: ok\n", r.Command)
	} else {
		fmt.Fprintf(w, "%s: failed\n", r.Command)
	}
	for _, d := range r.Warnings {
		fmt.Fprintf(w, "warning [%s] %s\n", d.Code, d.Message)
	}
	for _, d := range r.Errors {
		fmt.Fprintf(w, "error [%s] %s\n", d.Code, d.Message)
		if d.Hint != "" {
			fmt.Fprintf(w, "  hint: %s\n", d.Hint)
		}
	}
	if r.Data != nil {
		data, err := json.MarshalIndent(r.Data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "data:\n%s\n", data)
	}
	return nil
}
