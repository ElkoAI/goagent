# LLM Options for GoAgent

GoAgent works with any OpenAI-compatible LLM provider. Set three env vars and you're running:

```bash
LLM_BASE_URL=https://...   # provider's OpenAI-compatible endpoint
LLM_API_KEY=your-key        # API key (or any string for local)
LLM_MODEL=model-name        # model identifier
```

---

## 1. Cloud Providers

Ranked from free/most capable to paid/most capable. All support streaming (SSE) and the `/v1/chat/completions` endpoint GoAgent uses.

### Free Tier (no credit card)

| Provider          | Base URL                                                   | Best Free Model                                            | RPD               | Notes                                        |
| ----------------- | ---------------------------------------------------------- | ---------------------------------------------------------- | ----------------- | -------------------------------------------- |
| **Groq**          | `https://api.groq.com/openai/v1`                           | `meta-llama/llama-4-maverick-17b-128e-instruct` (400B MoE) | 1,000             | Fastest inference. Best free option overall. |
| **Cerebras**      | `https://api.cerebras.ai/v1`                               | `llama-4-scout-17b-16e-instruct`                           | ~1M tokens/day    | Extremely fast custom hardware.              |
| **Google Gemini** | `https://generativelanguage.googleapis.com/v1beta/openai/` | `gemini-2.5-flash`                                         | 1,000             | 1M token context window.                     |
| **SambaNova**     | `https://api.sambanova.ai/v1`                              | `Meta-Llama-3.1-405B-Instruct`                             | ~14,400           | Rare free access to a 405B model.            |
| **Mistral**       | `https://api.mistral.ai/v1`                                | Mistral Large                                              | ~86,400           | Phone verification required.                 |
| **OpenRouter**    | `https://openrouter.ai/api/v1`                             | `deepseek/deepseek-chat-v3-0324:free`                      | 50 (1,000 w/ $10) | 30+ free models from many providers.         |

#### Groq Setup (recommended)

```bash
# .env
LLM_BASE_URL=https://api.groq.com/openai/v1
LLM_API_KEY=gsk_your-key-here        # from console.groq.com
LLM_MODEL=meta-llama/llama-4-maverick-17b-128e-instruct
```

Sign up at [console.groq.com](https://console.groq.com) with GitHub/Google. No credit card. Key is instant.

Other strong Groq models: `llama-3.3-70b-versatile`, `moonshotai/kimi-k2-instruct`, `llama-3.1-8b-instant` (14,400 RPD).

#### Google Gemini Setup

```bash
# .env
LLM_BASE_URL=https://generativelanguage.googleapis.com/v1beta/openai/
LLM_API_KEY=your-gemini-api-key      # from aistudio.google.com
LLM_MODEL=gemini-2.5-flash
```

#### Cerebras Setup

```bash
# .env
LLM_BASE_URL=https://api.cerebras.ai/v1
LLM_API_KEY=your-cerebras-key        # from cloud.cerebras.ai
LLM_MODEL=llama-4-scout-17b-16e-instruct
```

### Free Tier (credits-based)

| Provider        | Base URL                              | Notes                                                     |
| --------------- | ------------------------------------- | --------------------------------------------------------- |
| **Together.ai** | `https://api.together.xyz/v1`         | Free credits on signup. 200+ models. Depletes with usage. |
| **NVIDIA NIM**  | `https://integrate.api.nvidia.com/v1` | 1,000 inference credits. Phone verification.              |

### Paid (most capable)

| Provider      | Base URL                       | Top Model                    | Approx. Cost         |
| ------------- | ------------------------------ | ---------------------------- | -------------------- |
| **OpenAI**    | `https://api.openai.com/v1`    | `gpt-4o`                     | $2.50-10/M tokens    |
| **Anthropic** | `https://api.anthropic.com/v1` | `claude-sonnet-4-5-20250929` | $3-15/M tokens       |
| **DeepSeek**  | `https://api.deepseek.com`     | `deepseek-chat`              | ~$0.27-1.10/M tokens |

DeepSeek is the default provider in GoAgent — strong model at very low cost.

---

## 2. Local Models (32GB System RAM)

### RAM Rule of Thumb

```
RAM needed ~ Model File Size + KV Cache + ~1-2 GB overhead
```

| Quantization | GB per Billion Params | 7B      | 14B    | 32B    | 70B    |
| ------------ | --------------------- | ------- | ------ | ------ | ------ |
| Q8_0         | ~1.1                  | ~8.5 GB | ~15 GB | ~35 GB | ~77 GB |
| Q5_K_M       | ~0.75                 | ~5.7 GB | ~10 GB | ~24 GB | ~53 GB |
| Q4_K_M       | ~0.65                 | ~4.9 GB | ~9 GB  | ~20 GB | ~42 GB |
| Q3_K_M       | ~0.5                  | ~3.7 GB | ~7 GB  | ~16 GB | ~35 GB |
| Q2_K         | ~0.4                  | ~3.0 GB | ~5 GB  | ~13 GB | ~26 GB |

Context length adds to this. At 4K context: +0.5-1 GB. At 32K context: +4-8 GB.

**Q4_K_M** is the sweet spot — best quality/size tradeoff. **Q5_K_M** when RAM allows.

### Best Models for 32GB RAM

#### Daily Driver: Qwen3-8B (~8.5 GB at Q8_0)

Best all-around small model. Thinking/non-thinking modes, strong at reasoning, math, coding. Tons of headroom — run at Q8_0 with large context.

```bash
# .env
LLM_BASE_URL=http://localhost:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:8b
```

#### Best Quality: Qwen3-14B (~9 GB at Q4_K_M, ~10.4 GB at Q5_K_M)

Significant quality jump over 8B. 128K context, 119 languages. Fits comfortably with 16K+ context.

```bash
LLM_MODEL=qwen3:14b
```

#### Maximum Capability: Qwen3-30B-A3B (~18.6 GB at Q4_K_M)

MoE model — 30.5B total params but only 3.3B active per token. Inference speed similar to a small model despite the large file. ~13 GB headroom for context.

```bash
LLM_MODEL=qwen3:30b-a3b
```

#### Coding Specialist: Qwen3-Coder-30B-A3B (~18.6 GB at Q4_K_M)

Same MoE architecture, optimized for code. 256K native context.

#### Reasoning: DeepSeek-R1-Distill-Qwen-14B (~10.4 GB at Q5_K_M)

Chain-of-thought reasoning. Outperforms OpenAI o1-mini on many benchmarks.

```bash
LLM_MODEL=deepseek-r1:14b
```

#### Other Good Options

| Model                    | Q4_K_M Size | Best For              |
| ------------------------ | ----------- | --------------------- |
| Llama 3.1 8B Instruct    | ~4.9 GB     | General, 128K context |
| Mistral 7B Instruct v0.3 | ~4.4 GB     | Fast, creative tasks  |
| Gemma 2 9B Instruct      | ~5.5 GB     | Research              |
| Phi-3.5 Mini (3.8B)      | ~2.4 GB     | Tiny but capable      |

### Can 70B Models Run on 32GB?

Technically at Q2_K (~26 GB) a 70B loads, but it's **not recommended**: context limited to ~2K tokens, ~0.5-1 tok/sec on CPU, noticeable quality loss. A 14B at Q5_K_M will outperform a 70B at Q2_K in both speed and coherence.

---

## 3. Local LLM Runners

### Ollama (recommended for GoAgent)

Wraps llama.cpp in a simple CLI with built-in model management. One command install, one command run.

```bash
# Install
curl -fsSL https://ollama.com/install.sh | sh

# Pull and run a model
ollama pull qwen3:14b
ollama serve                   # starts API at :11434

# GoAgent .env
LLM_BASE_URL=http://localhost:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:14b
```

**Model management:**

```bash
ollama pull <model>            # download
ollama list                    # show downloaded models
ollama run <model>             # interactive chat (also downloads)
ollama rm <model>              # delete
ollama ps                      # show running models
```

**Pros:** Dead simple, auto-detects GPU, manages downloads.
**Cons:** 13-80% slower than raw llama.cpp, less control over context/GPU settings.

### llama.cpp (maximum performance)

The C/C++ engine everything else builds on. Direct control over quantization, context, GPU offloading.

```bash
# Install
# Option A: Homebrew
brew install llama.cpp

# Option B: Build from source (CPU)
git clone https://github.com/ggml-org/llama.cpp.git
cd llama.cpp
cmake -B build && cmake --build build --config Release

# Option C: Build with CUDA GPU support
cmake -B build -DGGML_CUDA=ON
cmake --build build --config Release

# Option D: Build with Vulkan GPU support (AMD/Intel/NVIDIA)
cmake -B build -DGGML_VULKAN=ON
cmake --build build --config Release
```

**Start the OpenAI-compatible server:**

```bash
# Basic — auto-download from HuggingFace
llama-server \
  -hf bartowski/Qwen_Qwen3-14B-GGUF:Q4_K_M \
  --host 0.0.0.0 --port 8080 \
  -c 8192

# With GPU offloading (all layers to GPU)
llama-server \
  -m /path/to/model.gguf \
  --host 0.0.0.0 --port 8080 \
  -c 8192 -ngl 99

# Large context with KV cache quantization (saves RAM)
llama-server \
  -m /path/to/model.gguf \
  --host 0.0.0.0 --port 8080 \
  -c 32768 -ngl 99 -fa \
  -ctk q4_0 -ctv q4_0
```

**Key flags:**

| Flag           | Purpose                                         |
| -------------- | ----------------------------------------------- |
| `-m`           | Path to GGUF model file                         |
| `-hf`          | Download from HuggingFace (`user/repo:quant`)   |
| `-c`           | Context size in tokens (default 4096)           |
| `-ngl`         | Layers to offload to GPU (99 = all)             |
| `-fa`          | Enable Flash Attention                          |
| `-ctk`, `-ctv` | KV cache quantization (reduces context RAM ~4x) |
| `-np`          | Parallel request slots                          |

```bash
# GoAgent .env
LLM_BASE_URL=http://localhost:8080/v1
LLM_API_KEY=no-key
LLM_MODEL=local-model
```

**Pros:** Fastest inference, full GPU control, KV cache quantization, direct HuggingFace downloads.
**Cons:** Manual model management, more flags to learn.

### llama.cpp vs Ollama: When to Use Which

|                      | Ollama                       | llama.cpp                               |
| -------------------- | ---------------------------- | --------------------------------------- |
| **Setup time**       | 1 minute                     | 5-15 minutes                            |
| **Performance**      | Good                         | Best (13-80% faster)                    |
| **GPU control**      | Automatic                    | Full (`-ngl`, `--tensor-split`)         |
| **Context tuning**   | Limited (default ~11K)       | Full (`-c`, `-ctk`, `-ctv`)             |
| **Model management** | Built-in (`pull`/`rm`)       | Manual or `-hf` flag                    |
| **Multi-model**      | Hot-swaps models             | One model per server                    |
| **Best for**         | Getting started, convenience | Production, large context, benchmarking |

**Recommendation:** Start with Ollama. Switch to llama.cpp when you need more control or better performance.

### Other Runners

#### LM Studio (GUI + API)

Desktop app with model browser, chat UI, and headless API server. Good for exploring models before committing to one.

```bash
# API server at http://localhost:1234/v1
LLM_BASE_URL=http://localhost:1234/v1
```

#### vLLM (production GPU serving)

High-throughput engine using PagedAttention. 2-4x throughput over llama.cpp for concurrent requests. Needs 16GB+ VRAM. Overkill for single-user dev work.

```bash
docker run --gpus all -p 8000:8000 vllm/vllm-openai \
  --model meta-llama/Meta-Llama-3.1-8B-Instruct
# LLM_BASE_URL=http://localhost:8000/v1
```

#### LocalAI (full OpenAI replacement)

Drop-in replacement for the entire OpenAI API surface — text, images, audio, embeddings. Multiple backends (llama.cpp, vLLM, Transformers). Best when you need more than just chat completions.

```bash
docker run -p 8080:8080 localai/localai
# LLM_BASE_URL=http://localhost:8080/v1
```

#### llamafile (zero-install)

Mozilla's single-file executable that bundles model + runtime. Double-click to run. Best for demos and distribution, not for ongoing dev work (large files, slower than native llama.cpp).

#### GPT4All (desktop + local RAG)

Desktop app with built-in RAG over local documents. Good for non-technical users. API at `http://localhost:4891/v1`.

### Compatibility Summary

All runners below expose `/v1/chat/completions` and work with GoAgent:

| Runner    | Default Port | Base URL                    |
| --------- | ------------ | --------------------------- |
| Ollama    | 11434        | `http://localhost:11434/v1` |
| llama.cpp | 8080         | `http://localhost:8080/v1`  |
| LM Studio | 1234         | `http://localhost:1234/v1`  |
| vLLM      | 8000         | `http://localhost:8000/v1`  |
| LocalAI   | 8080         | `http://localhost:8080/v1`  |
| GPT4All   | 4891         | `http://localhost:4891/v1`  |

---

## Quick-Start: Fastest Path to a Working Local LLM

```bash
# 1. Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# 2. Pull a model
ollama pull qwen3:14b

# 3. Set .env
cat >> .env << 'EOF'
LLM_BASE_URL=http://localhost:11434/v1
LLM_API_KEY=ollama
LLM_MODEL=qwen3:14b
EOF

# 4. Start GoAgent and chat
./bin/goagent-gw &
./bin/goagent-cli --token=test123 chat advisor "Hello"
```
