package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

func resolveAddress(flagAddress string) (string, error) {
	address := strings.TrimSpace(flagAddress)
	if address == "" {
		address = defaultAddress
	}
	if portText := strings.TrimSpace(os.Getenv("PORT")); portText != "" && flagAddress == defaultAddress {
		port, err := strconv.Atoi(portText)
		if err != nil || port < 1 || port > 65535 {
			return "", errors.New("PORT 必须是 1 至 65535 的端口号")
		}
		address = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("监听地址必须为 host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("监听端口无效")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "", errors.New("拒绝默认暴露到全部网络接口，请明确使用回环地址")
	}
	return address, nil
}
