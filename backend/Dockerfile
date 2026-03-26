FROM golang:1.24-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN mkdir -p /etc/gateway && cp /app/config.yml /etc/gateway/config.yml
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /bin/app ./cmd/rate-limiter

FROM gcr.io/distroless/static-debian12:nonroot AS runner
COPY --from=builder /bin/app /bin/app
COPY --from=builder /etc/gateway/config.yml /etc/gateway/config.yml
ENTRYPOINT ["/bin/app", "--config", "/etc/gateway/config.yml"]
