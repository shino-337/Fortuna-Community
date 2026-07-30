FROM golang:1.22 AS builder
WORKDIR /app
cat << 'EOF' > main.go
package main

import "fmt"

func main() {
    fmt.Println("hello word")
}
EOF
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /hello main.go

FROM gcr.io/distroless/static-debian12
COPY --from=builder /hello /hello
ENTRYPOINT ["/hello"]
