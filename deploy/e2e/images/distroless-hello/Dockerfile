FROM golang:1.22 AS builder
WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /hello main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /hello /hello
ENTRYPOINT ["/hello"]
