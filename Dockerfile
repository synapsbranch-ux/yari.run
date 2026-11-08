# Builds one Go service of this repository: docker build --build-arg CMD=mcp .
# Used by docker-compose.yml for the MCP server. Not a production image.
FROM golang:1.27.3-bookworm AS build
ARG CMD
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "$CMD" && CGO_ENABLED=0 go build -trimpath -o /out/service ./cmd/${CMD}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/service /service
USER nonroot:nonroot
ENTRYPOINT ["/service"]
