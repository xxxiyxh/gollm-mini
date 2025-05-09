package server

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"

	"gollm-mini/internal/core"
	"gollm-mini/internal/types"
)

type ChatRequest struct {
	Messages []types.Message `json:"messages" binding:"required"`
	Provider string          `json:"provider" default:"ollama"`
	Model    string          `json:"model"    default:"llama3"`
	Schema   string          `json:"schema"`           // 为空则普通文本
	Stream   bool            `json:"stream,omitempty"` // SSE 是否流式
}

type ChatResponse struct {
	Text   string      `json:"text,omitempty"`
	JSON   interface{} `json:"json,omitempty"`
	Usage  types.Usage `json:"usage"`
	ErrMsg string      `json:"error,omitempty"`
}

func Run(ctx context.Context, addr string) error {
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 主聊天端点
	r.POST("/chat", func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		llm, err := core.New(req.Provider, req.Model)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 非流式、无 schema —— 普通文本
		if !req.Stream && req.Schema == "" {
			text, usage, err := llm.Generate(ctx, req.Messages)
			c.JSON(http.StatusOK, ChatResponse{
				Text:   text,
				Usage:  usage,
				ErrMsg: errMsg(err),
			})
			return
		}

		// 结构化 JSON（强制非流式）
		if req.Schema != "" {
			var out map[string]interface{}
			usage, err := llm.StructuredGenerate(ctx, req.Messages, req.Schema, &out)
			c.JSON(http.StatusOK, ChatResponse{
				JSON:   out,
				Usage:  usage,
				ErrMsg: errMsg(err),
			})
			return
		}

		// 流式 SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		flusher, _ := c.Writer.(http.Flusher)

		_, err = llm.Stream(ctx, req.Messages, func(ch types.Chunk) {
			_ = writeSSE(c.Writer, "data", ch.Content)
			flusher.Flush()
		})

		_ = writeSSE(c.Writer, "event", "done")
		if err != nil {
			_ = writeSSE(c.Writer, "error", err.Error())
		}
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	return srv.ListenAndServe()
}

// ---- helpers ----

func writeSSE(w http.ResponseWriter, field, data string) error {
	_, err := w.Write([]byte(field + ": " + data + "\n\n"))
	return err
}

func errMsg(e error) string {
	if e != nil {
		return e.Error()
	}
	return ""
}
