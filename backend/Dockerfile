# Dependencies are vendored, so the build needs no network.
# Build a static binary, then ship it with only CA certificates (to call the bank gateway over HTTPS).
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY vendor ./vendor
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /backend . \
 && mkdir /data && chown 65534:65534 /data

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /backend /backend
COPY --from=build --chown=65534:65534 /data /data
USER 65534:65534
ENV DATA_FILE=/data/state.json
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD ["/backend", "-healthcheck"]
ENTRYPOINT ["/backend"]
