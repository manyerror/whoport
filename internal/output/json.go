package output

import (
	"encoding/json"
	"io"
)

// Result 是一次终止尝试的结果，供 --json 输出，方便脚本消费。
type Result struct {
	Port    uint16 `json:"port"`
	PID     int32  `json:"pid"`
	Process string `json:"process,omitempty"`
	Source  string `json:"source,omitempty"`

	// Status 取值：killed / dry_run / refused / failed / not_occupied
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// WriteJSON 输出缩进过的 JSON。ports.Listener 自带 json tag，可直接序列化。
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
