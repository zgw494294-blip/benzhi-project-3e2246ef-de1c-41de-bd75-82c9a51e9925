package main

import "testing"

func TestConfigDefaultsAndPORT(t *testing.T) {
	configuration, err := parseConfig(nil, func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:19081" {
		t.Fatalf("默认地址为 %s", configuration.address)
	}
	configuration, err = parseConfig(nil, func(name string) string {
		if name == "PORT" {
			return "19123"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if configuration.address != "127.0.0.1:19123" {
		t.Fatalf("PORT 地址为 %s", configuration.address)
	}
}

func TestConfigRejectsInvalidPORT(t *testing.T) {
	if _, err := parseConfig(nil, func(string) string { return "8080x" }); err == nil {
		t.Fatal("应拒绝非法 PORT")
	}
}
