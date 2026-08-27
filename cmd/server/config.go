package main

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const defaultAddress = "127.0.0.1:19081"

type config struct {
	address   string
	dataDir   string
	selfcheck bool
}

func parseConfig(args []string, getenv func(string) string) (config, error) {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	var result config
	flags.StringVar(&result.address, "addr", "", "HTTP 监听地址")
	flags.StringVar(&result.dataDir, "data", "./data", "本地持久化目录")
	flags.BoolVar(&result.selfcheck, "selfcheck", false, "运行公开 API 端到端自检后退出")
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	if flags.NArg() != 0 {
		return config{}, fmt.Errorf("存在未识别的位置参数")
	}
	if result.address == "" {
		portValue := strings.TrimSpace(getenv("PORT"))
		if portValue == "" {
			result.address = defaultAddress
		} else {
			port, err := strconv.Atoi(portValue)
			if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portValue {
				return config{}, fmt.Errorf("PORT 必须是 1 至 65535 的十进制端口号")
			}
			result.address = net.JoinHostPort("127.0.0.1", portValue)
		}
	}
	if err := validateAddress(result.address); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(result.dataDir) == "" {
		return config{}, fmt.Errorf("-data 不能为空")
	}
	return result, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 必须为 host:port：%w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return fmt.Errorf("-addr 端口无效")
	}
	if host == "" {
		return fmt.Errorf("-addr 必须明确指定监听主机")
	}
	return nil
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || (ip != nil && ip.IsLoopback())
}
