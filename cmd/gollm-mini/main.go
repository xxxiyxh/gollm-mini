package main

import (
	"context"
	"flag"
	"fmt"
	"gollm-mini/internal/server"
	"net/http"
	"os"
	"time"

	// side‑effect import：目前只有 Ollama，后续可追加 _ "…/openai"
	_ "gollm-mini/internal/provider/ollama"

	"gollm-mini/internal/cli"
)

func main() {
	// --------- CLI 参数解析 ---------
	mode := flag.String("mode", "chat", "运行模式：chat / server / rag ...")
	provider := flag.String("provider", "ollama", "Provider：ollama / openai ...")
	model := flag.String("model", "llama3", "模型名称：llama3 / gpt-4o-mini ...")
	stream := flag.Bool("stream", true, "是否实时输出（结构化 JSON 会自动关闭）")
	schemaPath := flag.String("schema", "", "JSON Schema 文件路径（触发结构化模式）")

	port := flag.String("port", "8080", "server 端口")

	timeout := flag.Duration("timeout", 5*time.Minute, "全局超时时间")
	flag.Parse()

	// 结构化模式自动关闭流式
	if *schemaPath != "" {
		*stream = false
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	switch *mode {
	case "chat":
		err := cli.RunChat(ctx, *provider, *model, *schemaPath, *stream)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "server":
		fmt.Println("REST server :%" + *port)
		if err := server.Run(ctx, ":"+*port); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "未知 mode: %s\n", *mode)
		os.Exit(1)
	}
}
