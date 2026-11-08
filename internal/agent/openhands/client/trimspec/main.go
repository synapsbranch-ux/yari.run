// Command trimspec derives the small OpenAPI 3.0 document in ../openapi.yaml from
// the full OpenAPI 3.1 document served by the OpenHands agent server
// (GET /openapi.json, ~700 KB, 500+ schemas).
//
// Why: the full document is huge and uses 3.1 constructs (anyOf with null, const,
// numeric exclusiveMinimum, discriminated unions) that make the generated client
// unusable and noisy. yari only drives a handful of endpoints, so this tool keeps
// those operations, flattens the schema constructs oapi-codegen handles poorly and
// replaces heavy sub-schemas with free-form objects.
//
// Usage (see ../doc.go for the full regeneration recipe):
//
//	go run ./internal/agent/openhands/client/trimspec -in full-openapi.json -out internal/agent/openhands/client/openapi.yaml
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

type obj = map[string]any

// operation selects one operation of the full spec and gives it a stable name.
type operation struct {
	path, method, id string
	// successCodes are the response codes of the trimmed spec. The server documents 200
	// for POST /api/conversations but actually answers 201 on create (200 when the
	// conversation id already exists), so both are declared.
	successCodes []string
}

var operations = []operation{
	{"/health", "get", "Health", []string{"200"}},
	{"/ready", "get", "Ready", []string{"200"}},
	{"/server_info", "get", "ServerInfo", []string{"200"}},
	{"/api/conversations", "post", "StartConversation", []string{"200", "201"}},
	{"/api/conversations/{conversation_id}", "get", "GetConversation", []string{"200"}},
	{"/api/conversations/{conversation_id}", "delete", "DeleteConversation", []string{"200"}},
	{"/api/conversations/{conversation_id}/events", "post", "SendMessage", []string{"200"}},
	{"/api/conversations/{conversation_id}/events/search", "get", "SearchEvents", []string{"200"}},
	{"/api/conversations/{conversation_id}/events/count", "get", "CountEvents", []string{"200"}},
	{"/api/conversations/{conversation_id}/pause", "post", "PauseConversation", []string{"200"}},
	{"/api/conversations/{conversation_id}/run", "post", "RunConversation", []string{"200"}},
	{"/api/conversations/{conversation_id}/interrupt", "post", "InterruptConversation", []string{"200"}},
}

// dropParams are query/header parameters that yari never sends.
var dropParams = map[string]bool{
	"include_skills":                                true,
	"X-OpenHands-Observability-Span-Name":           true,
	"X-OpenHands-Observability-Tags":                true,
	"X-OpenHands-Observability-Metadata":            true,
	"X-OpenHands-Observability-Parent-Span-Context": true,
}

// keepProps restricts a schema to the listed properties (everything else is dropped).
// Schemas not listed here keep all their properties.
var keepProps = map[string][]string{
	"StartConversationRequest": {"workspace", "conversation_id", "confirmation_policy", "initial_message",
		"max_iterations", "stuck_detection", "tags", "autotitle", "agent", "max_budget_per_run"},
	"Agent-Input": {"llm", "tools", "mcp_config", "filter_tools_regex", "include_default_tools",
		"system_prompt_kwargs", "tool_concurrency_limit", "kind"},
	"LLM-Input": {"model", "api_key", "base_url", "usage_id", "timeout", "num_retries", "retry_multiplier",
		"retry_min_wait", "retry_max_wait", "max_output_tokens", "temperature", "top_p", "extra_headers",
		"litellm_extra_body", "drop_params", "caching_prompt", "native_tool_calling", "reasoning_effort",
		"stream", "max_message_chars", "stream_idle_timeout"},
	"MCPServer-Input": {"url", "transport", "command", "args", "env", "headers", "cwd", "timeout",
		"sse_read_timeout", "keep_alive", "enabled", "description"},
	"ServerInfo": {"uptime", "idle_time", "title", "version", "sdk_version", "tools_version",
		"workspace_version", "build_git_sha", "build_git_ref", "python_version", "usable_tools",
		"conversation_runtime"},
}

// opaque schemas are replaced by a free-form object (map[string]interface{} in Go).
var opaque = map[string]bool{
	"ConfirmationPolicyBase-Input": true,
	"BaseWorkspace":                true,
	"AgentBase-Output":             true,
	"ACPAgent-Input":               true,
}

// overrides are hand-written replacements for schemas whose real definition is untyped
// (events) or far larger than yari needs (ConversationInfo). The free-form tail
// (additionalProperties: true) keeps every other field available.
var overrides = map[string]obj{
	"Event": {
		"type":                 "object",
		"required":             []any{"id", "kind", "source", "timestamp"},
		"additionalProperties": true,
		"properties": obj{
			"id":        obj{"type": "string"},
			"kind":      obj{"type": "string"},
			"source":    obj{"type": "string"},
			"timestamp": obj{"type": "string"},
		},
	},
	"EventPage": {
		"type":     "object",
		"required": []any{"items"},
		"properties": obj{
			"items":        obj{"type": "array", "items": obj{"$ref": "#/components/schemas/Event"}},
			"next_page_id": obj{"type": "string"},
		},
	},
	"ConversationInfo": {
		"type":                 "object",
		"required":             []any{"id", "execution_status"},
		"additionalProperties": true,
		"properties": obj{
			"id":               obj{"type": "string"},
			"execution_status": obj{"$ref": "#/components/schemas/ConversationExecutionStatus"},
			"persistence_dir":  obj{"type": "string"},
			"max_iterations":   obj{"type": "integer"},
			"stuck_detection":  obj{"type": "boolean"},
			"title":            obj{"type": "string"},
			"created_at":       obj{"type": "string"},
			"updated_at":       obj{"type": "string"},
			"tags":             obj{"type": "object", "additionalProperties": obj{"type": "string"}},
			"workspace":        obj{"type": "object", "additionalProperties": true},
			"agent":            obj{"type": "object", "additionalProperties": true},
			"stats":            obj{"type": "object", "additionalProperties": true},
		},
	},
}

// propOverrides replace one property of one schema after conversion.
var propOverrides = map[string]map[string]obj{
	// The server accepts a oneOf (Agent | ACPAgent); yari only ever sends Agent.
	"StartConversationRequest": {"agent": {"$ref": "#/components/schemas/Agent-Input"}},
	// Text only; images are not used.
	"SendMessageRequest": {"content": {"type": "array", "items": obj{"$ref": "#/components/schemas/TextContent"}}},
}

type trimmer struct {
	src     obj
	schemas obj // source components.schemas
	out     obj // output components.schemas
	pending []string
	seen    map[string]bool
}

func main() {
	in := flag.String("in", "", "full OpenAPI JSON from GET /openapi.json")
	out := flag.String("out", "openapi.yaml", "trimmed OpenAPI YAML output")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "trimspec: -in is required")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*in)
	check(err)
	var src obj
	check(json.Unmarshal(raw, &src))
	sum := sha256.Sum256(raw)

	t := &trimmer{src: src, schemas: dig(src, "components", "schemas"), out: obj{}, seen: map[string]bool{}}
	paths := obj{}
	for _, op := range operations {
		srcOp := dig(src, "paths", op.path, op.method)
		if srcOp == nil {
			check(fmt.Errorf("operation %s %s not found in the full spec", op.method, op.path))
		}
		item, _ := paths[op.path].(obj)
		if item == nil {
			item = obj{}
			paths[op.path] = item
		}
		item[op.method] = t.operation(op, srcOp)
	}
	for len(t.pending) > 0 {
		name := t.pending[0]
		t.pending = t.pending[1:]
		t.out[rename(name)] = t.schema(name)
	}

	info, _ := src["info"].(obj)
	doc := obj{
		"openapi": "3.0.3",
		"info": obj{
			"title": "OpenHands agent server (trimmed for yari)",
			"description": fmt.Sprintf("Generated by trimspec from the agent server %v OpenAPI document (sha256 %s). Do not edit by hand.",
				info["version"], hex.EncodeToString(sum[:])),
			"version": fmt.Sprint(info["version"]),
		},
		"paths": paths,
		"components": obj{
			"securitySchemes": obj{"APIKeyHeader": obj{"type": "apiKey", "in": "header", "name": "X-Session-API-Key"}},
			"schemas":         t.out,
		},
	}
	b, err := yaml.Marshal(doc)
	check(err)
	header := "# Code generated by internal/agent/openhands/client/trimspec; DO NOT EDIT.\n"
	check(os.WriteFile(*out, append([]byte(header), b...), 0o644))
	fmt.Printf("wrote %s: %d operations, %d schemas\n", *out, len(operations), len(t.out))
}

func (t *trimmer) operation(op operation, src obj) obj {
	res := obj{"operationId": op.id, "security": []any{obj{"APIKeyHeader": []any{}}}}
	if s, ok := src["summary"].(string); ok {
		res["summary"] = s
	}
	var params []any
	for _, p := range asSlice(src["parameters"]) {
		pm, _ := p.(obj)
		if dropParams[fmt.Sprint(pm["name"])] {
			continue
		}
		np := obj{"name": pm["name"], "in": pm["in"]}
		if req, _ := pm["required"].(bool); req || pm["in"] == "path" {
			np["required"] = true
		}
		np["schema"] = t.convert(pm["schema"])
		params = append(params, np)
	}
	if len(params) > 0 {
		res["parameters"] = params
	}
	if rb, ok := src["requestBody"].(obj); ok {
		sch := dig(rb, "content", "application/json", "schema")
		if sch != nil {
			// Drop the inline `examples` that sit next to the $ref.
			delete(sch, "examples")
			res["requestBody"] = obj{"required": true, "content": obj{"application/json": obj{"schema": t.convert(sch)}}}
		}
	}
	responses := obj{}
	srcResp := dig(src, "responses", "200")
	schema := obj(nil)
	if srcResp != nil {
		schema = dig(srcResp, "content", "application/json", "schema")
	}
	for _, code := range op.successCodes {
		r := obj{"description": "OK"}
		if op.id == "SearchEvents" {
			r["content"] = obj{"application/json": obj{"schema": t.convert(obj{"$ref": "#/components/schemas/EventPage"})}}
		} else if schema != nil {
			r["content"] = obj{"application/json": obj{"schema": t.convert(schema)}}
		}
		responses[code] = r
	}
	responses["default"] = obj{"description": "Error (FastAPI detail body)"}
	res["responses"] = responses
	return res
}

func (t *trimmer) schema(name string) any {
	if ov, ok := overrides[name]; ok {
		return t.convert(deepCopy(ov))
	}
	node := deepCopy(t.schemas[name]).(obj)
	if keep, ok := keepProps[name]; ok {
		props, _ := node["properties"].(obj)
		kept := obj{}
		for _, k := range keep {
			if v, ok := props[k]; ok {
				kept[k] = v
			}
		}
		node["properties"] = kept
		if req := asSlice(node["required"]); req != nil {
			var nr []any
			for _, r := range req {
				if _, ok := kept[fmt.Sprint(r)]; ok {
					nr = append(nr, r)
				}
			}
			if len(nr) > 0 {
				node["required"] = nr
			} else {
				delete(node, "required")
			}
		}
	}
	for prop, ov := range propOverrides[name] {
		if props, ok := node["properties"].(obj); ok {
			props[prop] = deepCopy(ov)
		}
	}
	return t.convert(node)
}

// convert rewrites a 3.1 schema node into the 3.0 subset the generator copes with.
func (t *trimmer) convert(n any) any {
	switch v := n.(type) {
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = t.convert(v[i])
		}
		return out
	case obj:
		if ref, ok := v["$ref"].(string); ok {
			name := strings.TrimPrefix(ref, "#/components/schemas/")
			if opaque[name] {
				return obj{"type": "object", "additionalProperties": true}
			}
			t.want(name)
			return obj{"$ref": "#/components/schemas/" + rename(name)}
		}
		for _, key := range []string{"anyOf", "oneOf"} {
			if alts, ok := v[key].([]any); ok {
				return t.collapse(v, key, alts)
			}
		}
		out := obj{}
		for k, val := range v {
			switch k {
			case "title", "description", "examples", "example", "default", "discriminator", "writeOnly",
				"readOnly", "deprecated", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength",
				"minItems", "maxItems", "pattern", "contentMediaType", "contentEncoding", "$schema":
				continue
			case "format":
				// Keep generated types plain: uuid/date-time/password all become strings.
				continue
			case "const":
				out["enum"] = []any{val}
			case "properties":
				props := obj{}
				for pk, pv := range val.(obj) {
					props[pk] = t.convert(pv)
				}
				out[k] = props
			default:
				out[k] = t.convert(val)
			}
		}
		if _, ok := out["enum"]; ok && out["type"] == nil {
			out["type"] = "string"
		}
		return out
	default:
		return n
	}
}

// collapse reduces anyOf/oneOf to a single branch: null branches are dropped, a single
// remaining branch is used as is, several branches become a free-form schema.
func (t *trimmer) collapse(parent obj, key string, alts []any) any {
	var rest []any
	for _, a := range alts {
		if am, ok := a.(obj); ok && am["type"] == "null" {
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) == 1 {
		return t.convert(rest[0])
	}
	// Multiple string-ish branches (e.g. plain and password strings) are still a string.
	allString := len(rest) > 0
	for _, a := range rest {
		if am, ok := a.(obj); !ok || am["type"] != "string" {
			allString = false
		}
	}
	if allString {
		return obj{"type": "string"}
	}
	return obj{}
}

func (t *trimmer) want(name string) {
	if t.seen[name] {
		return
	}
	if _, ok := overrides[name]; !ok && t.schemas[name] == nil {
		check(fmt.Errorf("schema %q referenced but not found", name))
	}
	t.seen[name] = true
	t.pending = append(t.pending, name)
}

func rename(name string) string {
	name = strings.TrimSuffix(name, "-Input")
	return strings.TrimSuffix(name, "-Output")
}

func dig(n obj, keys ...string) obj {
	cur := n
	for _, k := range keys {
		next, ok := cur[k].(obj)
		if !ok {
			return nil
		}
		cur = next
	}
	return cur
}

func asSlice(v any) []any { s, _ := v.([]any); return s }

func deepCopy(v any) any {
	switch x := v.(type) {
	case obj:
		o := make(obj, len(x))
		for k, val := range x {
			o[k] = deepCopy(val)
		}
		return o
	case []any:
		o := make([]any, len(x))
		for i := range x {
			o[i] = deepCopy(x[i])
		}
		return o
	default:
		return v
	}
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "trimspec:", err)
		os.Exit(1)
	}
}
