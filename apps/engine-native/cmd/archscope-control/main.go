// archscope-control talks only to an explicitly enabled local desktop instance.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"os"
	"time"
)

func main() {
	descriptor := flag.String("descriptor", "", "private descriptor path")
	id := flag.String("id", "", "unique request ID (reuse only for identical retries)")
	command := flag.String("command", "status", "control command")
	params := flag.String("params", "{}", "JSON command parameters")
	flag.Parse()
	if *descriptor == "" || *id == "" || !json.Valid([]byte(*params)) {
		fmt.Fprintln(os.Stderr, "descriptor, ID and valid JSON parameters are required")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	reply, err := automation.Invoke(ctx, *descriptor, automation.Call{ID: *id, Command: *command, Params: json.RawMessage(*params)})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(reply)
	if !reply.OK {
		os.Exit(1)
	}
}
