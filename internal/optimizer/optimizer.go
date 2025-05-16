package optimizer

import (
	"context"
	"fmt"
	"time"

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
) (
	best template.Template,
	scores map[string]float64,
	answers map[string]string,
	err error,
) {
	const judgeSys = "你是评分助手，针对给定回答(Answer)和问题(Question)按照相关性与流畅度打分1-10，只回复数字。"

	store, _ := Open("optimize.db")
	scores = make(map[string]float64)
	answers = make(map[string]string)

	history := []types.Message{{Role: types.RoleSystem, Content: judgeSys}}
	question := vars["input"]

	for _, tpl := range templates {
		key := tplKey(tpl)

		// 1. 渲染模板
		msgs, renderErr := tpl.Render(vars, nil, "")
		if renderErr != nil {
			err = renderErr
			return
		}

		// 2. 模型生成回答
		answer, _, genErr := llm.Generate(ctx, msgs)
		if genErr != nil {
			err = genErr
			return
		}
		answers[key] = answer

		// 3. 构造评分 Prompt 并获取分数
		prompt := fmt.Sprintf("Question:%s\nAnswer:%s\nScore:", question, answer)
		scoreTxt, _, scoreErr := llm.Generate(ctx, append(history, types.Message{Role: types.RoleUser, Content: prompt}))
		if scoreErr != nil {
			err = scoreErr
			return
		}
		score := helper.ParseFloat(scoreTxt)
		scores[key] = score

		// 4. Prometheus 记录
		monitor.OptScore.WithLabelValues(llm.Provider(), tpl.Name).Observe(score)

		// 5. 落库每条评分记录
		_ = store.Save(Record{
			Template: key,
			Input:    question,
			Answer:   answer,
			Score:    score,
			Provider: llm.Provider(),
			Model:    llm.Model(),
			At:       time.Now(),
		})
	}

	// 6. 找得分最高者
	var max float64
	for k, v := range scores {
		if v >= max {
			max = v
			best = findTplByKey(templates, k)
		}
	}

	return
}

func tplKey(t template.Template) string {
	return fmt.Sprintf("%s:%d", t.Name, t.Version)
}

func findTplByKey(arr []template.Template, key string) template.Template {
	for _, t := range arr {
		if tplKey(t) == key {
			return t
		}
	}
	return template.Template{}
}
