# syntax=docker/dockerfile:1
#
# yari agent sandbox image.
#
# Base: the OpenHands Software Agent SDK "agent server", pinned by tag AND digest.
# Never use `latest`. To bump: pick a stable semver "-python" tag from
# https://github.com/OpenHands/software-agent-sdk/pkgs/container/agent-server,
# update both the tag and the digest (`docker buildx imagetools inspect <tag>`),
# then re-run the hardening matrix in agent/image/README.md.
FROM ghcr.io/openhands/agent-server:1.60.1-python@sha256:b43ee580c2177280c81cb807c9a559d7a14aa13b48dba5040132a6a637bb6d55

USER root

# Verified against the pinned base (see README.md): it already ships git, make, gcc,
# g++, build-essential, curl, wget, jq, tmux, patch, ca-certificates, python3, uv and
# node, so nothing is installed by default and this layer builds without network.
# Extra packages for a given deployment can be added with
#   --build-arg EXTRA_APT_PACKAGES="pkg-config unzip ripgrep"
# (needs apt mirror access at build time).
#
# The base also puts the runtime user in the `sudo` group with passwordless sudo.
# No session needs root, so remove sudo entirely (defence in depth on top of
# `--security-opt no-new-privileges`). `purge` works offline.
ARG EXTRA_APT_PACKAGES=""
RUN set -eux; \
    command -v git make gcc g++ cc curl >/dev/null; \
    test -f /etc/ssl/certs/ca-certificates.crt; \
    if [ -n "${EXTRA_APT_PACKAGES}" ]; then \
        apt-get update; \
        DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ${EXTRA_APT_PACKAGES}; \
        rm -rf /var/lib/apt/lists/*; \
    fi; \
    SUDO_FORCE_REMOVE=yes DEBIAN_FRONTEND=noninteractive apt-get purge -y --auto-remove sudo; \
    if command -v sudo >/dev/null; then echo "sudo still present" >&2; exit 1; fi; \
    (gpasswd -d openhands sudo || true)

# Server configuration. None of these values are secrets.
#   - Absolute state paths: the process working directory becomes /workspace below,
#     and the server's defaults are relative to it.
#   - VS Code and the browser preload are not used by yari sessions. The VS Code
#     server would otherwise listen on 0.0.0.0:8001 with the session key as its
#     connection token (visible in `ps`).
#   - LITELLM_LOCAL_MODEL_COST_MAP avoids a start-up fetch from raw.githubusercontent.com
#     (the sandbox has no internet).
# The session API key is NOT baked: pass OH_SESSION_API_KEYS_0 at `docker run` time.
ENV OH_CONVERSATIONS_PATH=/workspace/conversations \
    OH_BASH_EVENTS_DIR=/workspace/bash_events \
    OH_WORKSPACE_PATH=/workspace/project \
    OH_ENABLE_VSCODE=false \
    OH_PRELOAD_TOOLS=false \
    LITELLM_LOCAL_MODEL_COST_MAP=True \
    OPENHANDS_SUPPRESS_BANNER=1

# /workspace is created (owned by the runtime user) by the base image; make sure the
# state directories exist with the right owner so a fresh named volume inherits them.
RUN install -d -o openhands -g openhands -m 0755 \
        /workspace /workspace/project /workspace/conversations /workspace/bash_events

USER openhands
WORKDIR /workspace
EXPOSE 8000

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8000/health || exit 1

# ENTRYPOINT is inherited: tini -- /usr/local/bin/openhands-agent-server (listens on 0.0.0.0:8000).
