# gollm-mini

> **A minimal yet extensible Go SDK + service that lets you chat with local or cloud LLMs via CLI or REST/SSE.**

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue)](#) [![License](https://img.shields.io/badge/license-MIT-green)](#)

---

##  Why gollm‑mini?

* **Small & Clear** – the entire core fits in a handful of files, easy to read and hack.
* **Provider‑agnostic** – switch between **Ollama**, **OpenAI** (or add your own) with one flag.
* **Streaming or Batch** – real‑time token push via CLI or Server‑Sent Events.
* **Structured JSON** – guarantee responses comply with your JSON Schema, with auto‑retry.
* **Safety Nets** – context‑window truncation + exponential back‑off retries out of the box.

---

## Quick Start

```bash
go mod tidy  # fetch deps

# 1. local LLM via Ollama
gollm-mini -mode=chat -provider=ollama -model=llama3

# 2. cloud LLM via OpenAI
gollm-mini -mode=chat -provider=openai -model=gpt-4o-mini

# 3. run as REST server (SSE enabled)
gollm-mini -mode=server -port=8080
```

> **Prerequisites**
>
> * Go 1.21+
> * Ollama (for local inference) *or* an OpenAI key

---

## CLI Usage

```bash
gollm-mini \
  -mode=chat \
  -provider=ollama \
  -model=llama3 \
  -stream=true           # disable for single shot

# Structure output
# schema must be a local file, relative paths allowed

gollm-mini -mode=chat -schema=person.schema.json -stream=false
```

---

## REST API

### POST /health

Simple liveness probe.

### POST /chat

| Field      | Type        | Required | Description                                   |
| ---------- | ----------- | -------- | --------------------------------------------- |
| `messages` | `Message[]` | ✔        | chat history (role `system\|user\|assistant`) |
| `provider` | string      | –        | default `ollama`                              |
| `model`    | string      | –        | default `llama3`                              |
| `schema`   | path        | –        | enable structured JSON mode                   |
| `stream`   | bool        | –        | `true` to receive SSE chunks                  |

#### Example (cURL)

```bash
curl -N \
 -H "Content-Type: application/json" \
 -d '{
      "messages":[{"role":"user","content":"Say hi"}],
      "stream":true
    }' \
 http://localhost:8080/chat
```

---

## Configuration Options

| Flag        | Env | Default  | Description                          |
| ----------- | --- | -------- | ------------------------------------ |
| `-provider` | –   | `ollama` | API backend plugin                   |
| `-model`    | –   | `llama3` | model id for the provider            |
| `-schema`   | –   | –        | JSON Schema file for structured mode |
| `-stream`   | –   | `true`   | CLI streaming switch                 |
| `-port`     | –   | `8080`   | REST server port                     |

---


## Contributing

1. Fork & clone
2. `make dev` to run tests + linters
3. Submit a pull‑request – please follow [Conventional Commits](https://www.conventionalcommits.org/)

We welcome new providers, better tokenizers, example notebooks, docs translations…

---


