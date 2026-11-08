# Packages one prebuilt Go service of this repository: make service-binaries, then
#   docker build --build-arg CMD=mcp .
# docker-compose.yml uses it for the MCP server. Not a production image.
# The binary is built on the host (CGO off, linux) so the build needs no network access.
FROM gcr.io/distroless/static-debian12:nonroot
ARG CMD
COPY bin/linux/${CMD} /service
USER nonroot:nonroot
ENTRYPOINT ["/service"]
