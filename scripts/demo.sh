#!/usr/bin/env bash
# Creates an organization with a protected production environment through the local API, starts
# an agent session on it with the CLI, and streams the session until it ends.
# Needs: make up migrate agent-image, docker compose --profile agent up -d --wait,
# make api, make worker (in other terminals). Works with YARI_AGENT_RUNTIME=fake and no model key.
set -euo pipefail

API="${YARI_API_URL:-http://localhost:8080}"
SUBJECT="${YARI_DEV_SUBJECT:-demo}"
SLUG="demo-$(date +%s)"
export YARI_API_URL="$API" YARI_DEV_SUBJECT="$SUBJECT"

yari() { go run ./cmd/yari "$@"; }
call() { # service method json
  curl -fsS -X POST "$API/yari.v1.$1/$2" -H 'content-type: application/json' -H "X-Yari-Dev-Subject: $SUBJECT" -d "$3"
}
first_id() { grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4; }

echo ">> creating organization $SLUG"
yari orgs create "$SLUG" --name "Demo $SLUG"
ORG=$(call HierarchyService ListOrganizations '{}' | grep -o "\"id\":\"[^\"]*\",\"slug\":\"$SLUG\"" | cut -d'"' -f4)
WS=$(call HierarchyService CreateWorkspace "{\"organizationId\":\"$ORG\",\"slug\":\"main\",\"name\":\"Main\"}" | first_id)
PROJECT=$(call HierarchyService CreateProject "{\"workspaceId\":\"$WS\",\"slug\":\"shop\",\"name\":\"Shop\"}" | first_id)
ENV_ID=$(call HierarchyService CreateEnvironment "{\"projectId\":\"$PROJECT\",\"slug\":\"production\",\"name\":\"Production\"}" | first_id)
echo ">> production environment $ENV_ID (protected automatically)"

echo ">> starting the agent"
SESSION=$(yari agent start --env "$ENV_ID" "Make the sample build pass and deploy it" | awk 'NR==1{print $2}')
yari agent watch "$SESSION"
echo ">> open http://localhost:5173/approvals with PUBLIC_USE_FAKE_DATA=false to approve the plan"
