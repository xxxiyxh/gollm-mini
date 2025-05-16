package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gollm-mini/internal/core"
	"gollm-mini/internal/template"
	"gollm-mini/internal/types"
)

// RunChat 交互式 CLI
func RunChat(ctx context.Context,
	provider, model, schema, tplName, varJSON, sysOverride string,
	stream bool,
) error {

	// ---------- 1. 载入模板 ----------
	var (
		tpl       template.Template
		tplLoaded bool
		vars      map[string]string
	)
	if tplName != "" {
		store, err := template.Open("templates.db")
		if err != nil {
			return err
		}
		if tpl, err = store.Latest(tplName); err != nil {
			return err
		}
		tplLoaded = true
		_ = json.Unmarshal([]byte(varJSON), &vars)
		if vars == nil {
			vars = make(map[string]string)
		}
	}

	// ---------- 2. 创建 LLM ----------
	llm, err := core.New(provider, model)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🔹 gollm-mini | 交互模式，exit 退出")

	// ---------- 3. 初始化对话历史 ----------
	var history []types.Message
	if !tplLoaded { // 无模板时自行插入 System
		sys := sysOverride
		if sys == "" {
			sys = template.DefaultSystem
		}
		history = []types.Message{{Role: types.RoleSystem, Content: sys}}
	}

	// ---------- 4. 主循环 ----------
	for {
		fmt.Print("\n👤 > ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)
		if userInput == "exit" {
			return nil
		}

		// ----- 4.1 组装 prompt -----
		var messages []types.Message
		if tplLoaded {
			vars["input"] = userInput
			msgs, err := tpl.Render(vars, history, sysOverride)
			if err != nil {
				fmt.Println("Render Err:", err)
				continue
			}
			messages = msgs
		} else {
			messages = append(history,
				types.Message{Role: types.RoleUser, Content: userInput},
			)
		}

		// ----- 4.2 结构化输出 -----
		if schema != "" {
			var result map[string]interface{}
			if _, err := llm.StructuredGenerate(ctx, messages, schema, &result); err != nil {
				fmt.Println("Error：结构化失败:", err)
				continue
			}
			pretty, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println("🤖 JSON:\n", string(pretty))
			history = append(history,
				types.Message{Role: types.RoleAssistant, Content: string(pretty)},
				types.Message{Role: types.RoleUser, Content: userInput},
			)
			continue
		}

		// ----- 4.3 普通问答 -----
		if stream {
			var buf strings.Builder
			if _, err := llm.Stream(ctx, messages, func(ch types.Chunk) {
				fmt.Print(ch.Content)
				buf.WriteString(ch.Content)
			}); err != nil {
				fmt.Println("\nError:", err)
				continue
			}
			history = append(history,
				types.Message{Role: types.RoleAssistant, Content: buf.String()},
				types.Message{Role: types.RoleUser, Content: userInput},
			)
		} else {
			ans, _, err := llm.Generate(ctx, messages)
			if err != nil {
				fmt.Println("Error:", err)
				continue
			}
			fmt.Println("🤖:", ans)
			history = append(history,
				types.Message{Role: types.RoleAssistant, Content: ans},
				types.Message{Role: types.RoleUser, Content: userInput},
			)
		}
	}
}
