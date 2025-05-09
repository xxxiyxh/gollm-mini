package core

import (
	"context"
	"encoding/json"
	"time"

	"gollm-mini/internal/helper"
	"gollm-mini/internal/types"
)

const structuredRetries = 3

// StructuredGenerate 给定 schema & prompt，自动重试直到输出合法 JSON
func (l *LLM) StructuredGenerate(
	ctx context.Context,
	prompt []types.Message,
	schemaPath string,
	out interface{},
) (types.Usage, error) {

	var usage types.Usage
	err := Retry(ctx, structuredRetries, 300*time.Millisecond, func() error {
		// 1. 在系统指令前追加“严格输出 JSON”提示
		enforced := append(
			[]types.Message{{Role: types.RoleSystem, Content: "请仅以符合 schema 的 JSON 输出，勿添加解释。"}},
			prompt...,
		)

		txt, u, err := l.Generate(ctx, enforced)
		usage = u
		if err != nil {
			return err
		}

		if err := helper.ParseJSON(txt, out); err != nil {
			return err // 触发重试
		}
		// 二次验证 schema
		raw, _ := json.Marshal(out)
		return helper.ValidateJSONSchema(schemaPath, raw)
	})
	return usage, err
}
