# gollm-mini

> **A minimal, extensible LLM orchestration tool written in Go. Supports multiple providers, prompt templating, structured JSON responses, caching, prompt optimization, and streaming via CLI or REST/SSE.**

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](#) [![License](https://img.shields.io/badge/license-MIT-green)](#)

---

## ✨ Why gollm‑mini?

* **Minimalistic & Extensible:** Lightweight core built for clarity and easy customization.
* **Multiple Providers:** Seamlessly switch between **Ollama**, **OpenAI**, **HuggingFace**, or extend with your custom provider.
* **Prompt Management:** Structured templates with versioning, variable checks, context, directives, and output hints.
* **Prompt Optimization (A/B Testing):** Automatically compare prompts or models, score outputs, and select the optimal variant.
* **Caching:** High-performance prompt caching (SHA256 + BoltDB), reducing repeated calls and latency.
* **Structured JSON Outputs:** Ensure responses comply with predefined JSON schemas, automatically retry on validation failure.
* **Comprehensive Monitoring:** Built-in Prometheus metrics (latency, tokens, cost, cache hits) for easy integration with Grafana.
* **Robust & Safe:** Automatic context truncation, exponential backoff retries, and error handling out-of-the-box.

---

## 🚀 Quick Start

```bash
go mod tidy  # fetch dependencies

# Chat via CLI (Ollama local inference)
gollm-mini -mode=chat -provider=ollama -model=llama3

# Chat via CLI (OpenAI cloud inference)
OPENAI_API_KEY=<your-key> gollm-mini -mode=chat -provider=openai -model=gpt-4o-mini

# Run as REST/SSE server
gollm-mini -mode=server -port=8080

# Huggingface local server
# Install python3.12:
brew update
brew install python@3.12
echo 'export PATH="/usr/local/opt/python@3.12/bin:$PATH"' >> ~/.zshrc

# Create venv
python3.12 -m venv venv
source venv/bin/activate
pip install fastapi uvicorn transformers torch
```

---

## 🎛️ CLI Usage Examples

```bash
# Real-time streaming chat (default)
gollm-mini -mode=chat -provider=ollama -model=llama3

# Non-streaming mode
gollm-mini -mode=chat -stream=false

# Structured JSON output
# schema is a local JSON schema file path
gollm-mini -mode=chat -schema=person.schema.json -stream=false

# HuggingFace local service (Python)
# Start local HuggingFace service using uvicorn (recommended)
cd gollm-mini/providers/huggingface
uvicorn server:app --host 0.0.0.0 --port 8000 --reload
```

---

## 🌐 REST API

### ✅ **POST** `/health`

Simple liveness check.

### 💬 **POST** `/chat`

| Field      | Type        | Required | Description                                   |
| ---------- | ----------- | -------- | --------------------------------------------- |
| `messages` | `Message[]` | ✔        | chat history (role `system\|user\|assistant`) |
| `provider` | string      | –        | default `ollama`                              |
| `model`    | string      | –        | default `llama3`                              |
| `schema`   | path        | –        | JSON schema for structured mode               |
| `stream`   | bool        | –        | `true` for SSE streaming                      |

---

### ⚡ **POST** `/optimizer`

Compare and optimize prompts or providers.

```json
{
  "variants": [
    {"provider": "ollama", "model": "llama3", "tpl": "summary", "version": 1},
    {"provider": "openai", "model": "gpt-4o", "tpl": "summary", "version": 2}
  ],
  "vars": {"input": "Explain Go concurrency", "lang": "en"}
}
```

Returns `scores`, `answers`, `latencies`, and selects the optimal variant automatically.

---

## 📈 Monitoring & Metrics

Built-in Prometheus metrics include:

* **LLM Latency & Cost:** Track performance and expenses per provider/model.
* **Cache Hit/Miss:** Monitor caching efficiency.
* **Optimizer Scores:** Analyze prompt/model optimization results.

Easily visualize data using Grafana dashboards.

---

## 📚 Prompt Templates

Supports structured templates with context, directives, output hints, versioning, and variable checks.

```json
{
  "name": "summary",
  "version": 1,
  "content": "Summarize in {{.lang}}: {{.input}}",
  "vars": ["lang", "input"],
  "context": "You are an experienced tech writer.",
  "directives": "Avoid first-person voice.",
  "output_hint": "At least 100 words in markdown."
}
```

---

## 📦 Project Structure

```
gollm-mini/
├── internal/
│   ├── core/        # LLM call wrapper, caching, retries
│   ├── provider/    # Providers: Ollama, OpenAI, HuggingFace
│   ├── template/    # Prompt templating, variable validation
│   ├── optimizer/   # Prompt & model optimization, scoring, storage
│   ├── cache/       # BoltDB caching system
│   ├── monitor/     # Prometheus metrics integration
│   └── server/      # REST/SSE API handlers
└── cmd/gollm-mini/  # CLI & server entrypoints
```

---

## 🤝 Contributing

1. Fork & Clone
2. `make dev` to run tests and lint
3. Submit a PR following [Conventional Commits](https://www.conventionalcommits.org/)

We welcome new providers, improvements, examples, and documentation!

---

