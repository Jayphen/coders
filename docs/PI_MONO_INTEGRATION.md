# Pi-Mono Integration Analysis

Analysis of [badlogic/pi-mono](https://github.com/badlogic/pi-mono) components for potential integration with Coders.

**Pi-Mono Overview:**
- 2.2k+ stars, TypeScript-based AI agent toolkit
- 7 packages: LLM API, agent core, coding agent CLI, Slack bot, TUI, web UI, vLLM pods
- Similar problem space: building coding agents with good UX

## High-Value Integration Targets

### 1. **@mariozechner/pi-ai** - Multi-Provider LLM API ⭐⭐⭐⭐⭐

**What it does:**
- Unified API across OpenAI, Anthropic, Google, and other providers
- Single interface for different model backends
- Streaming support (likely, given it's a modern LLM wrapper)

**Integration value for Coders:**
- **CRITICAL**: Currently coders hardcodes Anthropic/Claude
- Ollama support is a manual env var hack (`CODERS_OLLAMA_*` → `ANTHROPIC_*`)
- This package would enable:
  - Native multi-model support (GPT-4, Gemini, Claude, etc.)
  - Easy model switching per session
  - Better Ollama integration without env var gymnastics
  - Future-proof against new providers

**Implementation path:**
- Go rewrite means we'd need to:
  - Port pi-ai TypeScript package to Go, OR
  - Use TypeScript shim/bridge, OR
  - Extract just the API patterns/interfaces
- Estimated effort: Medium (2-3 weeks for Go port)

**Priority: HIGH** - Solves real pain point in current architecture

---

### 2. **@mariozechner/pi-tui** - Terminal UI Library ⭐⭐⭐

**What it does:**
- Terminal UI with differential rendering
- Optimized for AI chat interfaces
- Custom components for agent interactions

**Integration value for Coders:**
- Currently using Bubbletea (Go) - works well
- Pi-tui is TypeScript, not directly compatible
- **BUT** could inspire improvements:
  - Differential rendering techniques
  - Better chat UI patterns
  - Component ideas for the dashboard

**Implementation path:**
- Study the rendering approach
- Port useful patterns to Bubbletea
- NOT a direct dependency

**Priority: LOW-MEDIUM** - Nice-to-have, not critical since Bubbletea is solid

---

### 3. **@mariozechner/pi-web-ui** - Web UI Components ⭐⭐⭐⭐

**What it does:**
- Web components for AI chat interfaces
- Browser-based UI for conversations
- Likely includes message rendering, streaming, etc.

**Integration value for Coders:**
- Current dashboard is minimal
- Could significantly enhance the web dashboard:
  - Better session chat views
  - Real-time streaming updates
  - Professional chat interface
  - Message history visualization

**Implementation path:**
- TypeScript/React compatible
- Could integrate directly into dashboard
- Coders already has TypeScript plugin infrastructure

**Priority: MEDIUM-HIGH** - Would greatly improve dashboard UX

---

### 4. **@mariozechner/pi-agent-core** - Agent Runtime ⭐⭐⭐

**What it does:**
- Agent runtime with tool calling
- State management for agents
- Autonomous behavior orchestration

**Integration value for Coders:**
- Coders has its own agent model (tmux + Claude sessions)
- Pi's agent core might offer:
  - Better tool calling abstractions
  - State management patterns
  - Agent coordination logic

**Trade-offs:**
- Coders architecture is quite different (session-based, not runtime-based)
- Might be heavyweight for our use case
- Redis already handles state

**Implementation path:**
- Study for patterns, likely not direct integration
- Extract useful state management concepts

**Priority: LOW** - Architecturally divergent

---

### 5. **@mariozechner/pi-mom** - Slack Bot ⭐⭐

**What it does:**
- Slack bot that delegates to coding agent
- Team collaboration integration

**Integration value for Coders:**
- Could enable team workflows:
  - Spawn sessions from Slack
  - Monitor session status
  - Get completion notifications
- Low priority for current roadmap

**Implementation path:**
- TypeScript, could work alongside plugin
- Moderate effort

**Priority: LOW** - Feature enhancement, not core need

---

### 6. **@mariozechner/pi-pods** - vLLM Pod Management ⭐

**What it does:**
- CLI for managing vLLM GPU deployments
- Infrastructure for self-hosted models

**Integration value for Coders:**
- Niche use case
- Most users use cloud APIs
- Ollama support already addresses self-hosting

**Priority: VERY LOW** - Not aligned with current use cases

---

## Recommended Integration Strategy

### Phase 1: Research & Prototype (1-2 weeks)
1. Clone pi-mono locally
2. Deep dive into `@mariozechner/pi-ai`:
   - How does the unified API work?
   - What's the interface design?
   - Can we extract just the multi-provider logic?
3. Prototype Go implementation or TypeScript bridge

### Phase 2: Multi-Provider Support (2-4 weeks)
1. Implement multi-provider LLM API in Go
2. Add model selection to spawn command:
   ```bash
   coders spawn --provider openai --model gpt-4
   coders spawn --provider anthropic --model claude-3-5-sonnet
   coders spawn --provider google --model gemini-2.0-pro
   ```
3. Update TUI to show provider/model info
4. Deprecate Ollama env var hack

### Phase 3: Dashboard Enhancement (2-3 weeks)
1. Integrate `@mariozechner/pi-web-ui` components
2. Build rich chat interface in dashboard
3. Add streaming message display
4. Improve session detail views

### Phase 4: Optional Enhancements
- Study pi-tui rendering patterns
- Consider Slack integration
- Evaluate agent-core patterns for future use

## Key Takeaways

**Most valuable:**
1. Multi-provider LLM API (solves current architectural limitation)
2. Web UI components (improves dashboard significantly)

**Worth studying:**
3. TUI rendering techniques
4. Agent state management patterns

**Not prioritized:**
5. Slack bot (nice-to-have)
6. vLLM pods (too niche)
7. Agent core (architectural mismatch)

## Technical Considerations

**Language barrier:**
- Pi-mono: TypeScript (96%)
- Coders: Go (packages/go) + TypeScript (plugin)
- Options:
  - Port to Go (clean, performant)
  - TypeScript bridge (faster, more complex)
  - Hybrid approach (Go core, TS UI)

**Licensing:**
- Need to check pi-mono license before integration
- Ensure compatibility with Coders distribution

**Maintenance:**
- Pi-mono is actively maintained (2.2k stars)
- Regular updates may require keeping in sync
- Consider: fork vs dependency vs inspiration
