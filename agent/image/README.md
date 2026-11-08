# yari agent sandbox image

The agent for a yari Agent Session is the **OpenHands Software Agent SDK**. Its **agent server**
image runs inside a hardened per-session sandbox container, and the Go worker (outside the sandbox)
drives it over REST and WebSocket. yari does not contain an agent loop or a model client.

Everything below was verified on 2026-11-08 by running the real image, reading its live
`/openapi.json`, and reading the server source at tag `v1.60.1`. Nothing is from memory.

Build: `docker build -t yari-agent:dev agent/image`

## 1. Pinned image

| | |
|---|---|
| Base image | `ghcr.io/openhands/agent-server:1.60.1-python` |
| Digest (multi-arch index) | `sha256:b43ee580c2177280c81cb807c9a559d7a14aa13b48dba5040132a6a637bb6d55` |
| Server / SDK / tools version | `1.60.1` (`GET /server_info`), git `e99e5c71ffe440a0b8d74b695f9307a7e34e4bbd` |
| OS / runtime in image | Debian 13 (trixie), Python 3.13, Node 22, `uv`; server is a PyInstaller binary `/usr/local/bin/openhands-agent-server` run under `tini` |
| Size | about 4.8 GB on disk |

`1.60.1-python` is the newest stable semver tag: probing the registry manifests found `1.60.1` and nothing
above it (`1.60.2` to `1.139.7` and `2.x` are absent; `1.60.0` was never published). The tag list that was
cached before this task stopped at `1.42.1` and the suggested `1.11.2` is months old, so do not trust a cached
list: probe the registry. Never use `latest` (it is a different, moving
digest). Bump by changing tag and digest together in `Dockerfile`, then re-run the matrix in section 6.

The base image already contains git, make, gcc, g++, build-essential, curl, wget, jq, tmux, patch,
ca-certificates, python3, pip, uv and node, so the `Dockerfile` installs nothing by default
(`--build-arg EXTRA_APT_PACKAGES="..."` adds packages when apt mirrors are reachable at build time).
It removes `sudo` (see traps below), bakes only non-secret `OH_*` settings, sets `WORKDIR /workspace`
and keeps the non-root user.

## 2. Create a conversation over REST

`POST /api/conversations` with header `X-Session-API-Key`. Response: `201` with the conversation
(`200` if the supplied `conversation_id` already exists). This exact body works and is the golden file
`internal/agent/openhands/client/testdata/start_conversation_request.json`:

```json
{
  "agent": {
    "kind": "Agent",
    "llm": {
      "usage_id": "yari-session",
      "model": "openai/yari-default",
      "base_url": "http://litellm.internal:4000/v1",
      "api_key": "sk-session-virtual-key-placeholder",
      "num_retries": 2
    },
    "tools": [
      {"name": "terminal"},
      {"name": "file_editor"},
      {"name": "task_tracker"}
    ],
    "mcp_config": {
      "yari": {
        "url": "http://yari-mcp.internal:8080/mcp",
        "transport": "streamable-http",
        "headers": {"Authorization": "Bearer yari-mcp-session-token-placeholder"}
      }
    }
  },
  "workspace": {"working_dir": "/workspace/project"},
  "max_iterations": 25,
  "autotitle": false,
  "stuck_detection": true
}
```

Verified details:

- **LLM**: `model` is a LiteLLM model string; with an OpenAI-compatible proxy use `openai/<name>`. The server
  sent `Authorization: Bearer <api_key>` to `<base_url>/chat/completions` (seen on a fake endpoint).
- **Tool names are snake_case**: `terminal`, `file_editor`, `task_tracker` (`GET /server_info` lists
  `usable_tools`). The CamelCase names in the OpenAPI examples (`TerminalTool`, ...) fail with HTTP 500
  `ToolDefinition 'TerminalTool' is not registered`. The agent also always gets a built-in `finish` tool.
  `browser_tool_set` is in the default preset; do not list it (Chromium needs `--no-sandbox` in a sandbox with
  no internet).
- **MCP**: `mcp_config` is a map of name to server. `headers` are sent on every MCP request (verified:
  `initialize`, `tools/list` and tool calls carry `Authorization: Bearer ...`). MCP tools register under
  their own name (a fake server's `yari_ping` appeared in the agent's tool list).
  Creation connects to the MCP server **synchronously**. If it is unreachable the server logs
  `MCP server startup failed for yari; continuing without its tools` and still returns 201, so the agent
  silently runs without the yari tools. The worker must verify the MCP tools are present (system prompt
  event `tools`) before trusting a session.
- **Iteration limit**: `max_iterations` (default 500). On reaching it the run ends with status `error` and a
  `ConversationErrorEvent` with `code: "MaxIterationsReached"`; the count is cumulative across pause/resume.
- **Secrets are never echoed**: the response has `llm.api_key: null` and drops MCP `headers`. On disk
  (`/workspace/conversations/<id>/base_state.json`) they are Fernet encrypted with `OH_SECRET_KEY`, which
  defaults to the session API key. Keep the session API key stable for the life of the session; a restarted
  container with the same key restored the conversation and kept working.
- **`initial_message` starts the run immediately, even with `"run": false`**, so the model is called as part
  of creation. To create without calling the model: omit `initial_message`, then `POST .../events` with
  `"run": false`, then `POST .../run` when ready. `autotitle: true` (default) makes an extra model call, so
  set it to `false`.
- Omit `agent_settings`/`agent_profile_id`; use the explicit `agent`. Pass `workspace.working_dir`.

## 3. Message, pause, resume, stop, delete

All under `/api/conversations/{id}` with `X-Session-API-Key`; success bodies are `{"success":true}`.

| Action | Request | Notes |
|---|---|---|
| Follow-up message | `POST /events` body `{"role":"user","content":[{"type":"text","text":"..."}],"run":true}` | `run:true` starts the loop if idle; `false` only stores the message. Works from `idle`, `finished`, `error`. |
| Pause | `POST /pause` | Waits for the in-flight LLM call and tool step to finish, then status `paused` and a `PauseEvent`. |
| Resume | `POST /run` | `409` if already running. Continues a `paused` conversation (and also restarts after `error`). |
| Stop now | `POST /interrupt` | Cancels the in-flight LLM request immediately (about 0 s vs a 15 s call); status `paused`, resumable. |
| Delete | `DELETE /{id}` | Works while running; later `GET` is `404`. |
| Status | `GET /{id}` | `execution_status`: `idle`, `running`, `paused`, `waiting_for_confirmation`, `finished`, `error`, `stuck`, `deleting`. |

Capacity: `OH_MAX_CONCURRENT_RUNS` (default 10), over it create/run return `429`.

## 4. Events

**List (REST)**: `GET /api/conversations/{id}/events/search?limit=100&page_id=<next_page_id>` returns
`{"items":[...],"next_page_id":"<event id>"|null}`. `limit` max 100. Filters: `kind`, `source`, `body`,
`timestamp__gte`, `timestamp__lt`, `sort_order` (`TIMESTAMP`, `TIMESTAMP_DESC`). `GET .../events/count`
takes the same filters. Pagination was verified with `limit=2`.

**Stream (WebSocket)**: `ws://host:8000/sockets/events/{conversation_id}` (use `wss` behind TLS).

- Auth, recommended: the **first frame** is `{"type":"auth","session_api_key":"<key>"}`. The upgrade is accepted
  first; a wrong key closes the socket with code `4001`. Also accepted: header `X-Session-API-Key` (non-browser)
  and deprecated query `?session_api_key=`. Unknown conversation closes with `4004`; too many subscribers `1013`.
- Query: `resend_mode=all` replays every stored event on connect, `resend_mode=since&after_timestamp=<ISO 8601>`
  replays newer ones. Without it only new events arrive.
- The first live frame on connect is a `ConversationStateUpdateEvent` with `key:"full_state"` (full state, secrets masked).
- Client to server: a JSON `{"role":"user","content":[{"type":"text","text":"..."}]}` frame is a user message and
  behaves like `run:true`. Bad frames produce a `ServerErrorEvent`.
- Frames are single JSON objects. Common fields: `id`, `timestamp` (naive ISO 8601, server local time,
  no zone), `source` (`user`, `agent`, `environment`), `kind`, `parent_id`.

| `kind` | source | Meaning and key fields |
|---|---|---|
| `SystemPromptEvent` | agent | First event; large (`system_prompt`, `tools`). |
| `MessageEvent` | user / agent | `llm_message.content[].text` |
| `ActionEvent` | agent | `tool_name`, `tool_call_id`, `action` (e.g. `TerminalAction` with `command`), `summary` |
| `ObservationEvent` | environment | `tool_name`, `action_id`, `observation` (`TerminalObservation`: `exit_code`, `content[].text`) |
| `AgentErrorEvent` | agent | Tool call invalid or failed |
| `ConversationStateUpdateEvent` | environment | `key` + `value`: `execution_status`, `stats`, `last_user_message_id`, `full_state` |
| `ConversationErrorEvent` | environment | Run aborted: `code`, `detail`, `classification {kind, retryable, user_action}` |
| `PauseEvent` / `InterruptEvent` | user | Pause or interrupt requested |
| `ServerErrorEvent` | environment | Socket handler rejected a client frame |

Fake-content samples of each are in `internal/agent/openhands/client/testdata/`. Go consumer:
`internal/agent/openhands/client/stream.go` (`Dial`, `Next`), exercised by `client_test.go` (hermetic) and
`integration_test.go` (real server).

## 5. Session API key, health

- Server env var: **`OH_SESSION_API_KEYS_0`** (indexed, `_1`, `_2`, ... for rotation). Legacy alias `SESSION_API_KEY`.
  With no key set the server is unauthenticated; always set one. Never put it in the image.
- Request header: **`X-Session-API-Key: <key>`** (the OpenAPI also lists `Authorization: Bearer` and a cookie scheme).
  `/api/*` returns `401` without it. Wrong key on the WebSocket: close `4001`.
- Unauthenticated by design: `GET /health` (`{"status":"ok"}`), `GET /alive`, `GET /ready` (`{"status":"ready"}`
  after init), `GET /server_info`, `GET /openapi.json`, `/docs`. Keep the port reachable only from the worker.
- The container `HEALTHCHECK` curls `/health`. Healthy about 5 to 10 s after start.
- Everything on `/api/*` is powerful (`/api/bash/execute_bash_command`, file upload/download, `/api/settings`).
  Whoever holds the key owns the sandbox. The agent itself can read the key from `/proc/1/environ`; it is
  session scoped, which is why it is the only secret allowed as container env.

## 6. Runtime facts and hardening (all tested)

- **User**: image default `openhands`, uid/gid **10001**. **Port**: **8000** (`0.0.0.0`). **Entrypoint**:
  `tini -- /usr/local/bin/openhands-agent-server`. **Working dir**: `/workspace` (this Dockerfile).
- **Writable paths needed** with a read-only root:
  - `/tmp` **with exec**: the PyInstaller binary unpacks and `mmap`s its libraries there. No `/tmp`: `[PYI-7:ERROR]
    Could not create temporary directory!`. `noexec`: `error while loading shared libraries: libz.so.1: failed to
    map segment`. Also used by tmux (`/tmp/openhands-agent-server-<pid>`).
  - `/home/openhands`: `~/.openhands`, `~/.config`, `~/.cache`. Without it creation fails with `Read-only file system:
    '/home/openhands/.openhands'`.
  - `/workspace`: `project/` (agent working dir), `conversations/` (persistence), `bash_events/`. Without it creation fails
    with `Read-only file system: '/workspace/conversations/<id>'`. Make it `exec` if sessions build and run code.
    Use a named volume (inherits uid 10001 from the image) or tmpfs with `uid=10001`.
- **Network**: the server needs no internet. At start LiteLLM tries to fetch a model price map from
  raw.githubusercontent.com and falls back to a bundled copy; `LITELLM_LOCAL_MODEL_COST_MAP=True` (baked) skips it.
  The sandbox network must still reach LiteLLM and the yari MCP server (attach them to the same internal network).

Tested one flag at a time on `yari-agent:dev` with a scripted conversation against a fake LLM, then combined:

| Flag | Result | Detail |
|---|---|---|
| non-root (`--user 10001:10001`) | PASS | default user |
| `--cap-drop ALL` | PASS | full terminal/file/git flow works; `CapEff=0` |
| `--security-opt no-new-privileges` | PASS | `NoNewPrivs: 1` |
| `--memory 2g --memory-swap 2g --cpus 2 --pids-limit 512` | PASS | cgroup shows `2147483648` and `512` |
| `--read-only` alone | FAIL | server does not start (needs `/tmp`) |
| `--read-only` + `/tmp` only | FAIL | conversation create fails (needs `/workspace`) |
| `--read-only` + `/tmp` + `/home/openhands` | FAIL | create fails (needs `/workspace`) |
| `--read-only` + `/tmp` + `/workspace` | FAIL | create fails (needs `/home/openhands`) |
| `--read-only` + `/home/openhands` + `/workspace` | FAIL | no `/tmp`: server does not start |
| `--read-only` + noexec `/tmp` | FAIL | cannot map shared libraries |
| `--read-only` + exec `/tmp` + `/home/openhands` + `/workspace` | PASS | |
| internal network (`docker network create --internal`) | PASS | no DNS, `curl https://example.com` and `http://1.1.1.1` fail; fake LLM/MCP on the same network reachable |
| **all of the above together** | **PASS** | scripted conversation: terminal call, file written, `finished`; Go integration test passes |

Working hardened run (no secret baked; `$SESSION_KEY` is the per-session value):

```sh
docker network create --internal yari-sbx-$SESSION_ID          # attach LiteLLM and yari-mcp to it too
docker run -d --name agent-$SESSION_ID --network yari-sbx-$SESSION_ID \
  --user 10001:10001 --read-only \
  --tmpfs /tmp:rw,exec,nosuid,nodev,size=512m \
  --tmpfs /home/openhands:rw,nosuid,nodev,size=256m,uid=10001,gid=10001,mode=0700 \
  --mount type=volume,src=yari-ws-$SESSION_ID,dst=/workspace \
  --cap-drop ALL --security-opt no-new-privileges \
  --memory 2g --memory-swap 2g --cpus 2 --pids-limit 512 \
  -e OH_SESSION_API_KEYS_0="$SESSION_KEY" \
  yari-agent:dev
```

Notes: an `--internal` network publishes no ports, so the worker must be attached to the network or call the
container IP. The default seccomp profile stays on. gVisor was not tested (runc only here).

### Traps found in the upstream image

1. **Passwordless `sudo` to root** for the runtime user (`sudo -n id` gives `uid=0`). `no-new-privileges` blocks
   it and so does `--cap-drop ALL`, but the `Dockerfile` also purges sudo so it does not depend on flags.
2. **VS Code server** (`OH_ENABLE_VSCODE=true` in the base) listens on `0.0.0.0:8001` with the **session API
   key as its connection token**, visible in `ps`. Baked off: `OH_ENABLE_VSCODE=false`. Only 8000 listens.
3. Default state paths are relative to the working directory; with `WORKDIR /workspace` they would land in
   `/workspace/workspace/...`. The `Dockerfile` sets absolute `OH_CONVERSATIONS_PATH`, `OH_BASH_EVENTS_DIR`,
   `OH_WORKSPACE_PATH`.
4. `OH_PRELOAD_TOOLS=false` (baked) skips the Chromium preload that needs a writable `~/.config/browseruse`.
5. The OpenAPI document says `200` for create (real: `201`), types event search as `{}` and uses 3.1-only
   constructs, which is why the Go client is generated from a trimmed spec.

## 7. Failure behaviour with an unreachable or fake model

Verified on the hardened container.

- **Create never needs the model.** With an unreachable `base_url` (`http://fake-llm.invalid:9999/v1`, or a
  closed port) and an unreachable MCP server, `POST /api/conversations` returned `201` in about 25 ms with
  `execution_status: "idle"` (no `initial_message`). The key is only used on the first model call.
- **Run fails gracefully.** `POST /events` with `run:true` (or `POST /run`) leads to `running`, then, after the
  configured retries (`llm.num_retries`, default 5 with 8 to 64 s backoff, so set it low for fast feedback),
  `execution_status: "error"` and a `ConversationErrorEvent`:
  `{"code":"LLMServiceUnavailableError","detail":"litellm.InternalServerError: ... Connection error.","classification":{"kind":"transient","retryable":true,"user_action":"retry"}}`.
  The server stays healthy, the conversation stays readable, and a later `POST /run` retries.
- HTTP 500 from a model endpoint behaves the same way as a refused connection.
- **Working fake**: against a local OpenAI-compatible endpoint that returns a `terminal` tool call and then a text
  message, the run reached `finished` with `ActionEvent` / `ObservationEvent` / `MessageEvent` and the file
  was written under `/workspace/project` as uid 10001.

## 8. Path decision: Path A, pure Go

The agent server is fully configurable over REST (LLM, base URL, key, tools, MCP with custom headers, iteration
cap, messages, pause/resume/interrupt/delete, event listing and WebSocket stream). No Python runner (Path B)
is needed.

- Client: `internal/agent/openhands/client`, generated by `oapi-codegen` (a `tool` directive in `go.mod`,
  `go tool oapi-codegen`) from `openapi.yaml`.
- OpenAPI 3.1 handling: oapi-codegen v2.8.2 parses the full 3.1 document, but it yields about 31,000 lines and
  unusable unions, so `openapi.yaml` is a **trimmed OpenAPI 3.0 spec** (12 operations, 16 schemas) derived by
  `internal/agent/openhands/client/trimspec` from the server's `/openapi.json`. The regeneration recipe is in
  `internal/agent/openhands/client/doc.go`.
- Events are untyped by design (`Event` keeps `id`, `kind`, `source`, `timestamp` plus all other fields as
  additional properties) because the server's event union is not in its OpenAPI document.
- WebSocket: `github.com/coder/websocket` in `stream.go`.
- Run the live check (needs a running hardened container):

```sh
YARI_TEST_OPENHANDS=1 OPENHANDS_TEST_URL=http://<container-ip>:8000 OPENHANDS_TEST_API_KEY=<session key> \
  go test ./internal/agent/openhands/client -run Integration -v
# optional: OPENHANDS_TEST_LLM_URL=http://<fake-llm>:9999/v1 for a model that answers
```
