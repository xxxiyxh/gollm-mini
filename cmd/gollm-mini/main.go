package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	// side‑effect 注册 Provider
	_ "gollm-mini/internal/provider/ollama"

	"gollm-mini/internal/cli"
	"gollm-mini/internal/server"
	"gollm-mini/internal/template"
)

func main() {
	// --------- CLI 参数解析 ---------
	mode := flag.String("mode", "chat", "运行模式：chat / server / template")
	provider := flag.String("provider", "ollama", "Provider：ollama / openai ...")
	model := flag.String("model", "llama3", "模型名称：llama3 / gpt-4o-mini ...")
	stream := flag.Bool("stream", true, "是否实时输出（结构化 JSON 会自动关闭）")
	schemaPath := flag.String("schema", "", "JSON Schema 文件路径（触发结构化模式）")

	port := flag.String("port", "8080", "server 端口")
	system := flag.String("system", "", "覆盖 system 指令文本")

	tplFlag := flag.String("tpl", "", "模板名称")
	varsFlag := flag.String("vars", "{}", "JSON 格式变量")

	timeout := flag.Duration("timeout", 5*time.Minute, "全局超时时间")
	flag.Parse()

	// ---------- 模板管理子命令 ----------
	if *mode == "template" {
		path := "templates.db"
		store, _ := template.Open(path)
		switch flag.Arg(0) {
		case "add":
			name := flag.Arg(1)
			file := flag.Arg(2) // .txt 文件
			b, _ := os.ReadFile(file)
			tpl := template.Template{
				Name:      name,
				Version:   1,
				System:    *system,
				Content:   string(b),
				Vars:      []string{"topic"},
				CreatedAt: time.Now(),
			}
			_ = store.Save(tpl)
			fmt.Println("saved", name)
			return
		case "list":
			// TODO: 遍历 bucket 打印
			return
		}
	}

	// 结构化模式自动关闭流式
	if *schemaPath != "" {
		*stream = false
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	switch *mode {
	case "chat":
		err := cli.RunChat(
			ctx,
			*provider,
			*model,
			*schemaPath,
			*tplFlag,
			*varsFlag,
			*system,
			*stream,
		)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	case "server":
		fmt.Println("REST server listening on :" + *port)
		if err := server.Run(ctx, ":"+*port); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "未知 mode: %s\n", *mode)
		os.Exit(1)
	}
}
