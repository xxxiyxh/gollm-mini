package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gollm-mini/internal/core"
	"gollm-mini/internal/types"
)

// RunChat 启动交互式对话
func RunChat(ctx context.Context, providerName, model, schema string, stream bool) error {

	// 创建单个 LLM（Provider + Model）
	llm, err := core.New(providerName, model)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("🔹 gollm-mini | 交互模式，exit 退出")

	history := []types.Message{
		{Role: types.RoleSystem, Content: "You are a helpful assistant."},
	}

	for {
		fmt.Print("\n👤 > ")
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)
		if user == "exit" {
			return nil
		}
		history = append(history, types.Message{Role: types.RoleUser, Content: user})

		// JSON 结构化输出
		if schema != "" {
			var result map[string]interface{}
			_, err := llm.StructuredGenerate(ctx, history, schema, &result)
			if err != nil {
				fmt.Println("Error： 结构化失败:", err)
				continue
			}
			pretty, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println("🤖 JSON:\n", string(pretty))
			history = append(history, types.Message{Role: types.RoleAssistant, Content: string(pretty)})
			continue // 跳过后续分支
		}

		//流式/非流式
		if stream {
			var buf strings.Builder
			_, err := llm.Stream(ctx, history, func(ch types.Chunk) {
				fmt.Print(ch.Content)
				buf.WriteString(ch.Content)
			})
			if err != nil {
				fmt.Println("\nError: ", err)
				continue
			}
			ans := buf.String()
			history = append(history, types.Message{Role: types.RoleAssistant, Content: ans})
		} else {
			ans, _, err := llm.Generate(ctx, history)
			if err != nil {
				fmt.Println("Error: ", err)
				continue
			}
			fmt.Println("🤖:", ans)
			history = append(history, types.Message{Role: types.RoleAssistant, Content: ans})
		}
	}
}
