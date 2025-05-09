package types

// Chunk 是 Streaming 输出的一次片段
type Chunk struct {
	Content string // 模型新生成的 token 文本
	Delta   int    // 本次片段新增 token 数
}
