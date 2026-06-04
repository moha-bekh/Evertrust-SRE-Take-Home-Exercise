FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/certificate-inspector ./cmd/server

FROM alpine:3.20

WORKDIR /app

COPY --from=builder /out/certificate-inspector /usr/local/bin/certificate-inspector
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/certificate-inspector"]
