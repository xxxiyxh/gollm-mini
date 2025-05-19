package template

import (
	"bytes"
	"fmt"
	texttemplate "text/template"

	"gollm-mini/internal/types"
)

func (t Template) Render(vars map[string]string, history []types.Message, sysOverride string) ([]types.Message, error) {
	for _, required := range t.Vars {
		if _, ok := vars[required]; !ok {
			return nil, fmt.Errorf("missing var: %s", required)
		}
	}
	tt, err := texttemplate.New("prompt").Parse(t.Content)
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

	userPrompt := buf.String()
	if t.Context != "" {
		userPrompt = fmt.Sprintf("%s\n\n%s", t.Context, userPrompt)
	}
	if t.Directives != "" {
		userPrompt = fmt.Sprintf("%s\n\n%s", userPrompt, t.Directives)
	}
	if t.OutputHint != "" {
		userPrompt = fmt.Sprintf("%s\n\n输出要求:%s", userPrompt, t.OutputHint)
	}
	msgs := []types.Message{
		{Role: types.RoleSystem, Content: systemText},
		{Role: types.RoleUser, Content: userPrompt},
	}
	// 追加历史
	msgs = append(history, msgs...)
	return msgs, nil
}
