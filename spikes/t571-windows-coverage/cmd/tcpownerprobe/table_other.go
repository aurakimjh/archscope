//go:build !windows

package main

import "fmt"

func pollTCP() ([]netTCPRow, error) {
	return nil, fmt.Errorf("GetExtendedTcpTable is only available on Windows")
}
