package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	fmt.Println("hello word")
	// Stay running so the pod remains in Running state; agent needs Running to enqueue SBOM.
	// E2E test can delete the pod with --cleanup. Default 10m; env SLEEP_SECONDS overrides.
	sec := 600
	if s := os.Getenv("SLEEP_SECONDS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			sec = n
		}
	}
	time.Sleep(time.Duration(sec) * time.Second)
}
