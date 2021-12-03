package peggo

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

func hexToBytes(str string) ([]byte, error) {
	str = strings.TrimPrefix(str, "0x")

	data, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// waitForService awaits an active connection to a gRPC service.
func waitForService(ctx context.Context, clientconn *grpc.ClientConn) {
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "gRPC service wait timed out")
			os.Exit(1)

		default:
			state := clientconn.GetState()

			if state != connectivity.Ready {
				fmt.Fprintf(os.Stderr, "state of gRPC connection not ready: %s\n", state)
				time.Sleep(5 * time.Second)
				continue
			}

			return
		}
	}
}
