# Build a static binary, then ship it alone: no Go, no shell, ~10 MB image.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mock-cbar .

FROM scratch
COPY --from=build /mock-cbar /mock-cbar
USER 65534:65534
EXPOSE 4000
HEALTHCHECK --interval=30s --timeout=3s CMD ["/mock-cbar", "-healthcheck"]
ENTRYPOINT ["/mock-cbar"]
