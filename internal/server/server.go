package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gollm-mini/internal/cache"
	"gollm-mini/internal/core"
	"gollm-mini/internal/memory"
	"gollm-mini/internal/optimizer"
	"gollm-mini/internal/template"
	"gollm-mini/internal/types"
)

/* ---------- request / response ---------- */

type ChatRequest struct {
	Messages  []types.Message   `json:"messages"`
	Tpl       string            `json:"tpl"`
	Vars      map[string]string `json:"vars"`
	System    string            `json:"system"`
	Provider  string            `json:"provider" default:"ollama"`
	Model     string            `json:"model"    default:"llama3"`
	Schema    string            `json:"schema"`
	Stream    bool              `json:"stream,omitempty"`
	SessionID string            `json:"session_id"` // 新增：对话记忆
}

type ChatResponse struct {
	Text   string      `json:"text,omitempty"`
	JSON   interface{} `json:"json,omitempty"`
	Usage  types.Usage `json:"usage"`
	ErrMsg string      `json:"error,omitempty"`
}

/* ---------- bootstrap ---------- */

func Run(ctx context.Context, addr string) error {
	r := gin.Default()

	tplStore, err := template.Open("templates.db")
	if err != nil {
		return err
	}

	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	chat := r.Group("/chat")
	{
		chat.POST("", func(c *gin.Context) { handleChat(c, tplStore) })
	}

	tpl := r.Group("/template")
	{
		tpl.POST("", func(c *gin.Context) { handleTplSave(c, tplStore) })
		tpl.GET("", func(c *gin.Context) { handleTplListAllLatest(c, tplStore) }) // NEW
		tpl.GET("/:name", func(c *gin.Context) { handleTplLatestOrVersions(c, tplStore) })
		tpl.GET("/:name/:ver", func(c *gin.Context) { handleTplGet(c, tplStore) })
		tpl.DELETE("/:name/:ver", func(c *gin.Context) { handleTplDel(c, tplStore) })
	}

	opt := r.Group("/optimizer")
	{
		opt.POST("", func(c *gin.Context) { handleOptimize(c, tplStore) })
	}

	cacheGrp := r.Group("/cache")
	{
		cacheGrp.DELETE("/all", handleCacheClearAll)
		cacheGrp.DELETE("/:key", handleCacheDelKey)
		cacheGrp.DELETE("/prefix/:prefix", handleCacheDelPrefix)
	}

	mem := r.Group("/memory")
	{
		mem.DELETE("/:sid", handleMemoryDelete) // DELETE /memory/{sid}
	}

	srv := &http.Server{Addr: addr, Handler: r}

	go func() { <-ctx.Done(); _ = srv.Shutdown(context.Background()) }()
	return srv.ListenAndServe()
}

/* ---------- chat ---------- */

func handleChat(c *gin.Context, tplStore *template.Store) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	llm, err := core.New(req.Provider, req.Model)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	/* ① 读取历史 */
	var history []types.Message
	if req.SessionID != "" {
		history, err = memory.Load(req.SessionID)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}

	/* ② 组装 prompt */
	msgs := req.Messages
	if len(msgs) == 0 && req.Tpl != "" {
		tpl, e := tplStore.Latest(req.Tpl)
		if e != nil {
			c.JSON(404, gin.H{"error": e.Error()})
			return
		}
		msgs, e = tpl.Render(req.Vars, history, req.System)
		if e != nil {
			c.JSON(400, gin.H{"error": e.Error()})
			return
		}
	}
	if len(history) > 0 && len(req.Messages) > 0 {
		msgs = append(history, req.Messages...)
	}
	if len(msgs) == 0 {
		c.JSON(400, gin.H{"error": "no messages or template provided"})
		return
	}

	/* ③ 非流式 & 无 schema */
	if !req.Stream && req.Schema == "" {
		text, usage, err := llm.Generate(c, msgs)
		c.JSON(200, ChatResponse{Text: text, Usage: usage, ErrMsg: errMsg(err)})

		if req.SessionID != "" && err == nil {
			_ = memory.Append(req.SessionID, []types.Message{
				{Role: types.RoleUser, Content: msgs[len(msgs)-1].Content},
				{Role: types.RoleAssistant, Content: text},
			})
		}
		return
	}

	/* ④ 结构化 JSON */
	if req.Schema != "" {
		var out map[string]interface{}
		usage, err := llm.StructuredGenerate(c, msgs, req.Schema, &out)
		c.JSON(200, ChatResponse{JSON: out, Usage: usage, ErrMsg: errMsg(err)})
		return
	}

	/* ⑤ 流式 SSE */
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	flusher, _ := c.Writer.(http.Flusher)

	var buf bytes.Buffer
	_, err = llm.Stream(c, msgs, func(ch types.Chunk) {
		_ = writeSSE(c.Writer, "data", ch.Content)
		buf.WriteString(ch.Content)
		flusher.Flush()
	})
	_ = writeSSE(c.Writer, "event", "done")
	if err != nil {
		_ = writeSSE(c.Writer, "error", err.Error())
	}

	if req.SessionID != "" && err == nil {
		_ = memory.Append(req.SessionID, []types.Message{
			{Role: types.RoleUser, Content: msgs[len(msgs)-1].Content},
			{Role: types.RoleAssistant, Content: buf.String()},
		})
	}
}

/* ---------- template CRUD ---------- */

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

func handleTplListAllLatest(c *gin.Context, store *template.Store) {
	list, err := store.ListAllLatest()
	if err != nil {
		c.JSON(500, err)
		return
	}
	c.JSON(200, list)
}

func handleTplLatestOrVersions(c *gin.Context, store *template.Store) {
	name := c.Param("name")
	if c.Query("all") == "1" {
		list, err := store.List(name)
		if err != nil {
			c.JSON(500, err)
			return
		}
		c.JSON(200, list)
		return
	}
	handleTplLatest(c, store) // 复用现有逻辑
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
func handleTplDel(c *gin.Context, store *template.Store) {
	v, _ := strconv.Atoi(c.Param("ver"))
	_ = store.Delete(c.Param("name"), v)
	c.Status(204)
}

/* ---------- optimizer ---------- */

func handleOptimize(c *gin.Context, store *template.Store) {
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))

	var req struct {
		Variants []optimizer.Variant `json:"variants"`
		Vars     map[string]string   `json:"vars"`
	}
	_ = json.Unmarshal(raw, &req)

	/* 兼容旧格式 */
	if len(req.Variants) == 0 {
		var legacy struct {
			Tpls []struct {
				Name    string
				Version int
			} `json:"tpls"`
			Vars            map[string]string `json:"vars"`
			Provider, Model string
		}
		_ = json.Unmarshal(raw, &legacy)
		for _, t := range legacy.Tpls {
			req.Variants = append(req.Variants, optimizer.Variant{
				Provider: legacy.Provider, Model: legacy.Model,
				TplName: t.Name, Version: t.Version,
			})
		}
		req.Vars = legacy.Vars
	}

	if len(req.Variants) == 0 {
		c.JSON(400, gin.H{"error": "variants required"})
		return
	}

	best, scores, answers, lat, err :=
		optimizer.RunVariants(c, req.Variants, req.Vars, store)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"best": best, "scores": scores, "answers": answers, "latencies": lat,
	})
}

/* ---------- cache handlers ---------- */

func handleCacheClearAll(c *gin.Context) {
	if err := cache.ClearAll(); err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
}
func handleCacheDelKey(c *gin.Context) {
	if err := cache.DeleteKey(c.Param("key")); err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
}
func handleCacheDelPrefix(c *gin.Context) {
	if err := cache.DeletePrefix(c.Param("prefix")); err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204)
	}
}

/* ---------- memory handlers ---------- */

func handleMemoryDelete(c *gin.Context) {
	if err := memory.Delete(c.Param("sid")); err != nil {
		c.JSON(500, err)
	} else {
		c.Status(204) // No Content
	}
}

/* ---------- helpers ---------- */

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
