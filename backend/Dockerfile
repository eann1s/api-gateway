FROM golang:1.24-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/app ./cmd/rate-limiter

FROM gcr.io/distroless/static-debian12:nonroot AS runner
COPY --from=builder /bin/app /bin/app
ENTRYPOINT ["/bin/app"]
