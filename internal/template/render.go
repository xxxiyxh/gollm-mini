package template

import (
	"bytes"
	"text/template"

	"gollm-mini/internal/types"
)

func (t Template) Render(vars map[string]string, history []types.Message, sysOverride string) ([]types.Message, error) {
	tt, err := template.New("prompt").Parse(t.Content)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tt.Execute(&buf, vars); err != nil {
		return nil, err
	}
	systemText := sysOverride
	if systemText == "" {
		if t.System != "" {
			systemText = t.System
		} else {
			systemText = DefaultSystem
		}
	}
	msgs := []types.Message{
		{Role: types.RoleSystem, Content: t.System},
		{Role: types.RoleUser, Content: buf.String()},
	}
	// 追加历史
	msgs = append(history, msgs...)
	return msgs, nil
}
