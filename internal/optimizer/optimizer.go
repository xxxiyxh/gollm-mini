package optimizer

import (
	"context"
	"fmt"

	"gollm-mini/internal/core"
	"gollm-mini/internal/helper"
	"gollm-mini/internal/monitor"
	"gollm-mini/internal/template"
	"gollm-mini/internal/types"
)

// SimpleA/B：多模板单模型对比，LLM 自评分 (1-10)
func RunAB(
	ctx context.Context,
	llm *core.LLM,
	templates []template.Template,
	vars map[string]string,
) (best template.Template, scores map[string]float64, answers map[string]string, err error) {

	const judgeSys = "你是评分助手，针对给定回答(Answer)和问题(Question)按照相关性与流畅度打分1-10，只回复数字。"

	scores = make(map[string]float64)
	answers = make(map[string]string)
	history := []types.Message{{Role: types.RoleSystem, Content: judgeSys}}

	question := vars["input"]

	for _, tpl := range templates {
		keys := tplKey(tpl)
		msgs, _ := tpl.Render(vars, nil, "")
		// 先得到该模板的回答
		answer, _, e := llm.Generate(ctx, msgs)
		if e != nil {
			err = e
			return
		}
		answers[keys] = answer

		// 让同一模型给出分数
		prompt := fmt.Sprintf("Question:%s\nAnswer:%s\nScore:", question, answer)
		scoreTxt, _, e := llm.Generate(ctx, append(history, types.Message{Role: types.RoleUser, Content: prompt}))
		if e != nil {
			err = e
			return
		}
		sc := helper.ParseFloat(scoreTxt) // 粗解析
		scores[tplKey(tpl)] = sc

		// Prometheus 记录
		monitor.OptScore.WithLabelValues(llm.Provider(), tpl.Name).Observe(sc)
	}

	// 选最高
	var max float64
	for k, v := range scores {
		if v >= max {
			max = v
			best = findTplByKey(templates, k)
		}
	}
	return
}

func tplKey(t template.Template) string { return fmt.Sprintf("%s:%d", t.Name, t.Version) }
func findTplByKey(arr []template.Template, key string) template.Template {
	for _, t := range arr {
		if tplKey(t) == key {
			return t
		}
	}
	return template.Template{}
}
