package server

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gollm-mini/internal/optimizer"
	"gollm-mini/internal/template"
	"net/http"
	"strconv"
	"time"

	"gollm-mini/internal/core"
	"gollm-mini/internal/types"
)

type ChatRequest struct {
	Messages []types.Message   `json:"messages"`
	Tpl      string            `json:"tpl"`
	Vars     map[string]string `json:"vars"`
	System   string            `json:"system"`
	Provider string            `json:"provider" default:"ollama"`
	Model    string            `json:"model"    default:"llama3"`
	Schema   string            `json:"schema"`
	Stream   bool              `json:"stream,omitempty"`
}

type ChatResponse struct {
	Text   string      `json:"text,omitempty"`
	JSON   interface{} `json:"json,omitempty"`
	Usage  types.Usage `json:"usage"`
	ErrMsg string      `json:"error,omitempty"`
}

func Run(ctx context.Context, addr string) error {
	r := gin.Default()

	tplStore, err := template.Open("templates.db")
	if err != nil {
		return err
	}

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

		// ---------- 组装 Prompt ----------
		msgs := req.Messages
		if len(msgs) == 0 && req.Tpl != "" {
			tpl, err := tplStore.Latest(req.Tpl)
			if err != nil {
				c.JSON(404, gin.H{"error": fmt.Sprintf("template %s not found", req.Tpl)})
				return
			}
			rendered, err := tpl.Render(req.Vars, nil, req.System)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			msgs = rendered
		}
		if len(msgs) == 0 {
			c.JSON(400, gin.H{"error": "no messages or template provided"})
			return
		}

		// 非流式、无 schema —— 普通文本
		if !req.Stream && req.Schema == "" {
			text, usage, err := llm.Generate(ctx, msgs)

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
			usage, err := llm.StructuredGenerate(ctx, msgs, req.Schema, &out)
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

		_, err = llm.Stream(ctx, msgs, func(ch types.Chunk) {
			_ = writeSSE(c.Writer, "data", ch.Content)
			flusher.Flush()
		})

		_ = writeSSE(c.Writer, "event", "done")
		if err != nil {
			_ = writeSSE(c.Writer, "error", err.Error())
		}
		return
	})

	r.POST("/template", func(c *gin.Context) { // 新增/覆盖
		var t template.Template
		if err := c.ShouldBindJSON(&t); err != nil {
			c.JSON(400, err)
			return
		}
		if t.System == "" {
			t.System = template.DefaultSystem
		}
		t.CreatedAt = time.Now()
		if err := tplStore.Save(t); err != nil {
			c.JSON(500, err)
			return
		}
		c.JSON(200, gin.H{"saved": t})
	})

	r.GET("/template/:name", func(c *gin.Context) { // latest
		t, err := tplStore.Latest(c.Param("name"))
		if err != nil {
			c.JSON(404, err)
			return
		}
		c.JSON(200, t)
	})

	r.GET("/template/:name/:ver", func(c *gin.Context) {
		v, _ := strconv.Atoi(c.Param("ver"))
		t, err := tplStore.Get(c.Param("name"), v)
		if err != nil {
			c.JSON(404, err)
			return
		}
		c.JSON(200, t)
	})

	r.DELETE("/template/:name/:ver", func(c *gin.Context) {
		v, _ := strconv.Atoi(c.Param("ver"))
		_ = tplStore.Delete(c.Param("name"), v)
		c.Status(204)
	})

	r.POST("/optimize", func(c *gin.Context) { // A/B 入口
		var req struct {
			Tpls []struct {
				Name    string `json:"name"`
				Version int    `json:"version"`
			} `json:"tpls" binding:"required"`
			Vars     map[string]string `json:"vars"`
			Provider string            `json:"provider"`
			Model    string            `json:"model"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, err)
			return
		}

		// Collect templates
		var tpls []template.Template
		for _, meta := range req.Tpls {
			t, err := tplStore.Get(meta.Name, meta.Version)
			if err != nil {
				c.JSON(404, gin.H{"error": fmt.Sprintf("template %s:%d not found", meta.Name, meta.Version)})
				return
			}
			tpls = append(tpls, t)
		}

		llm, err := core.New(req.Provider, req.Model)
		if err != nil {
			c.JSON(400, err)
			return
		}

		best, scores, answers, err := optimizer.RunAB(c, llm, tpls, req.Vars)
		if err != nil {
			c.JSON(500, err)
			return
		}
		c.JSON(200, gin.H{"best": best, "scores": scores, "answers": answers})
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
