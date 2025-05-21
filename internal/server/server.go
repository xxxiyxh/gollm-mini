package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gollm-mini/internal/cache"
	"gollm-mini/internal/optimizer"
	"gollm-mini/internal/template"
	"io"
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

	//Chat
	chat := r.Group("/chat")
	{
		chat.POST("", func(c *gin.Context) { handleChat(c, tplStore) })
	}

	//Template CRUD
	tpl := r.Group("/template")
	{
		tpl.POST("", func(c *gin.Context) { handleTplSave(c, tplStore) })
		tpl.GET("/:name", func(c *gin.Context) { handleTplLatest(c, tplStore) })
		tpl.GET("/:name/:ver", func(c *gin.Context) { handleTplGet(c, tplStore) })
		tpl.DELETE("/:name/:ver", func(c *gin.Context) { handleTplDel(c, tplStore) })
	}

	//Optimizer
	opt := r.Group("/optimizer")
	{
		opt.POST("", func(c *gin.Context) { handleOptimize(c, tplStore) })
	}

	//Cache
	cacheGrp := r.Group("/cache")
	{
		cacheGrp.DELETE("/all", handleCacheClearAll)
		cacheGrp.DELETE("/:key", handleCacheDelKey)
		cacheGrp.DELETE("/prefix/:prefix", handleCacheDelPrefix)
	}

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

func handleChat(c *gin.Context, tplStore *template.Store) {
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
		text, usage, err := llm.Generate(c.Request.Context(), msgs)

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
		usage, err := llm.StructuredGenerate(c.Request.Context(), msgs, req.Schema, &out)
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

	_, err = llm.Stream(c.Request.Context(), msgs, func(ch types.Chunk) {
		_ = writeSSE(c.Writer, "data", ch.Content)
		flusher.Flush()
	})

	_ = writeSSE(c.Writer, "event", "done")
	if err != nil {
		_ = writeSSE(c.Writer, "error", err.Error())
	}
	return
}

func handleTplSave(c *gin.Context, store *template.Store) {
	var t template.Template
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(400, err)
		return
	}
	if t.System == "" {
		t.System = template.DefaultSystem
	}
	t.CreatedAt = time.Now()
	if err := store.Save(t); err != nil {
		c.JSON(500, err)
		return
	}
	c.JSON(200, gin.H{"saved": t})
}

func handleTplLatest(c *gin.Context, store *template.Store) {
	t, err := store.Latest(c.Param("name"))
	if err != nil {
		c.JSON(404, err)
		return
	}
	c.JSON(200, t)
}

func handleTplDel(c *gin.Context, store *template.Store) {
	v, _ := strconv.Atoi(c.Param("ver"))
	_ = store.Delete(c.Param("name"), v)
	c.Status(204)
}

func handleTplGet(c *gin.Context, store *template.Store) {
	v, _ := strconv.Atoi(c.Param("ver"))
	t, err := store.Get(c.Param("name"), v)
	if err != nil {
		c.JSON(404, err)
		return
	}
	c.JSON(200, t)
}

func handleOptimize(c *gin.Context, store *template.Store) {
	// 0. 先一次性读出原始 body
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// 让 Body 可重复读（给 Gin 其他中间件）
	c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))

	// 1. 尝试新格式
	var reqNew struct {
		Variants []optimizer.Variant `json:"variants"`
		Vars     map[string]string   `json:"vars"`
	}
	_ = json.Unmarshal(raw, &reqNew)

	// 2. 若新格式为空，再尝试旧格式
	if len(reqNew.Variants) == 0 {
		var legacy struct {
			Tpls []struct {
				Name    string `json:"name"`
				Version int    `json:"version"`
			} `json:"tpls"`
			Vars     map[string]string `json:"vars"`
			Provider string            `json:"provider"`
			Model    string            `json:"model"`
		}
		if err := json.Unmarshal(raw, &legacy); err == nil && len(legacy.Tpls) > 0 {
			for _, t := range legacy.Tpls {
				reqNew.Variants = append(reqNew.Variants, optimizer.Variant{
					Provider: legacy.Provider,
					Model:    legacy.Model,
					TplName:  t.Name,
					Version:  t.Version,
				})
			}
			reqNew.Vars = legacy.Vars
		}
	}

	if len(reqNew.Variants) == 0 {
		c.JSON(400, gin.H{"error": "variants required"})
		return
	}

	best, scores, answers, lat, err :=
		optimizer.RunVariants(c, reqNew.Variants, reqNew.Vars, store)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"best":      best,
		"scores":    scores,
		"answers":   answers,
		"latencies": lat,
	})
}

func handleCacheClearAll(c *gin.Context) {
	err := cache.ClearAll()
	if err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
}

func handleCacheDelKey(c *gin.Context) {
	err := cache.DeleteKey(c.Param("key"))
	if err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
}

func handleCacheDelPrefix(c *gin.Context) {
	err := cache.DeletePrefix(c.Param("prefix"))
	if err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
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
