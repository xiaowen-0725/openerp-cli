// Package output owns the agent-facing wire contract: the success envelope, the
// typed error envelope, render formats (json|ndjson|table|csv), a small jq-path
// subset, and the Category→exit-code mapping. stdout carries data; stderr
// carries everything else (errors, verbose logs).
package output

// Envelope is the standard success wrapper written to stdout.
type Envelope struct {
	OK     bool                   `json:"ok"`
	Data   interface{}            `json:"data,omitempty"`
	Meta   *Meta                  `json:"meta,omitempty"`
	Notice map[string]interface{} `json:"_notice,omitempty"`
}

// Meta carries optional response metadata.
type Meta struct {
	Count int `json:"count,omitempty"`
}
