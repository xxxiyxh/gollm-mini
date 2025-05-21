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

type Variant struct {
	Provider string `json:"provider"`          // ollama / openai …
	Model    string `json:"model"`             // llama3 / gpt-4o …
	TplName  string `json:"tpl"`               // 模板名
	Version  int    `json:"version,omitempty"` // 模板版本，可 0
}

func (v Variant) Key() string {
	return fmt.Sprintf("%s|%s|%s:%d", v.Provider, v.Model, v.TplName, v.Version)
}

// RunVariants —— 跨 Provider / Model / Prompt 的统一对比入口
func RunVariants(
	ctx context.Context,
	variants []Variant,
	vars map[string]string,
	tplStore *template.Store, // 需要读模板
) (best Variant, scores map[string]float64,
	answers map[string]string, latencies map[string]float64, err error) {

	const judgeSys = "你是评分助手，针对给定回答(Answer)和问题(Question)按照相关性与流畅度打分1-10，只回复数字。"

	recDB, _ := Open("optimize.db") // 评分落库
	scores = map[string]float64{}
	answers = map[string]string{}
	latencies = map[string]float64{}

	question := vars["input"]
	judgePrompt := []types.Message{{Role: types.RoleSystem, Content: judgeSys}}

	for _, v := range variants {
		key := v.Key()

		// 1. 组装 Message
		tpl, e := tplStore.Get(v.TplName, v.Version)
		if e != nil {
			err = e
			return
		}
		msgs, e := tpl.Render(vars, nil, "")
		if e != nil {
			err = e
			return
		}

		// 2. 调用 LLM
		llm, e := core.New(v.Provider, v.Model)
		if e != nil {
			err = e
			return
		}

		start := time.Now()
		answer, _, e := llm.Generate(ctx, msgs)
		if e != nil {
			err = e
			return
		}
		lat := time.Since(start).Seconds()

		answers[key] = answer
		latencies[key] = lat
		monitor.CompareLatency.WithLabelValues(v.Provider).Observe(lat)

		// 3. 评分
		scorePrompt := fmt.Sprintf("Question:%s\nAnswer:%s\nScore:", question, answer)
		scoreTxt, _, e := llm.Generate(ctx, append(judgePrompt, types.Message{
			Role: types.RoleUser, Content: scorePrompt,
		}))
		if e != nil {
			err = e
			return
		}
		sc := helper.ParseFloat(scoreTxt)
		scores[key] = sc
		monitor.OptScore.WithLabelValues(v.Provider, v.TplName).Observe(sc)

		// 4. 落库
		_ = recDB.Save(Record{
			VariantKey: key,
			Input:      question,
			Answer:     answer,
			Score:      sc,
			Provider:   v.Provider,
			Model:      v.Model,
			At:         time.Now(),
		})
	}

	// 5. 选最优
	var max float64
	for _, v := range variants {
		if s := scores[v.Key()]; s >= max {
			max = s
			best = v
		}
	}

	return
}

// RunAB 同 Provider & Model，多模板对比
func RunAB(
	ctx context.Context,
	llm *core.LLM,
	tpls []template.Template,
	vars map[string]string,
	tplStore *template.Store, // ← 传入 store
) (best template.Template, scores map[string]float64,
	answers map[string]string, err error) {

	var variants []Variant
	for _, t := range tpls {
		variants = append(variants, Variant{
			Provider: llm.Provider(),
			Model:    llm.Model(),
			TplName:  t.Name,
			Version:  t.Version,
		})
	}

	bestVar, scores, answers, _, err :=
		RunVariants(ctx, variants, vars, tplStore) // ← 传递非 nil
	if err != nil {
		return
	}
	best = findTplByKey(tpls,
		fmt.Sprintf("%s:%d", bestVar.TplName, bestVar.Version))
	return
}

func findTplByKey(arr []template.Template, key string) template.Template {
	for _, t := range arr {
		if fmt.Sprintf("%s:%d", t.Name, t.Version) == key {
			return t
		}
	}
	return template.Template{}
}
