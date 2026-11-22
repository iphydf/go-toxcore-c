package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tox "github.com/TokTok/go-toxcore-c"
)

// settings are the user's choices for how the app connects and looks.
type settings struct {
	// UDP off connects through TCP relays only.
	UDP            bool `json:"udp"`
	IPv6           bool `json:"ipv6"`
	LocalDiscovery bool `json:"local_discovery"`
	// Proxy is "", "socks5" or "http": Tox connects through it.
	Proxy     string `json:"proxy,omitempty"`
	ProxyHost string `json:"proxy_host,omitempty"`
	ProxyPort uint16 `json:"proxy_port,omitempty"`
	// Nodes are bootstrap nodes of the user's own, a line each: host, port
	// and DHT key.
	Nodes string `json:"nodes,omitempty"`

	// Background keeps Tox connected while the app is closed, on platforms
	// that close apps.
	Background bool `json:"background"`
	Dark       bool `json:"dark,omitempty"`
	// TextSize scales the text: 1 is normal.
	TextSize float32 `json:"text_size,omitempty"`
}

// network is the part of the settings that Tox only reads when it starts.
func (s settings) network() settings {
	s.Background, s.Dark, s.TextSize = false, false, 0
	return s
}

func settingsPath(dir string) string { return filepath.Join(dir, "settings.json") }

// loadSettings reads the settings in dir, with toxcore's defaults for those
// the file doesn't have.
func loadSettings(dir string) (settings, error) {
	o := tox.DefaultOptions()
	s := settings{UDP: o.UDPEnabled, IPv6: o.IPv6Enabled, LocalDiscovery: o.LocalDiscoveryEnabled, Background: true, TextSize: 1}
	data, err := os.ReadFile(settingsPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	if s.TextSize == 0 {
		s.TextSize = 1
	}
	return s, err
}

func (s settings) save(dir string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(settingsPath(dir), append(data, '\n'))
}

// check says what's wrong with settings the user typed in.
func (s settings) check() error {
	if s.Proxy != "" && (s.ProxyHost == "" || s.ProxyPort == 0) {
		return errors.New("the proxy needs a host and a port")
	}
	_, err := parseNodes(s.Nodes)
	return err
}

func (s settings) apply(o *tox.Options) {
	o.UDPEnabled = s.UDP
	o.IPv6Enabled = s.IPv6
	o.LocalDiscoveryEnabled = s.LocalDiscovery
	switch s.Proxy {
	case "socks5":
		o.ProxyType = tox.ProxyTypeSOCKS5
	case "http":
		o.ProxyType = tox.ProxyTypeHTTP
	}
	if o.ProxyType != tox.ProxyTypeNone {
		o.ProxyHost, o.ProxyPort = s.ProxyHost, tox.Port(s.ProxyPort)
		// Through a proxy, Tox has only TCP.
		o.UDPEnabled = false
	}
}

// parseNodes reads bootstrap nodes, a line each: host, port and DHT key. The
// port is the UDP port and the TCP relay's.
func parseNodes(text string) ([]node, error) {
	var nodes []node
	for i, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			return nil, fmt.Errorf("node %d: a node is a host, a port and a key", i+1)
		}
		port, err := strconv.ParseUint(fields[1], 10, 16)
		if err != nil || port == 0 {
			return nil, fmt.Errorf("node %d: %q isn't a port", i+1, fields[1])
		}
		if _, err := tox.ParseDhtID(fields[2]); err != nil {
			return nil, fmt.Errorf("node %d: %q isn't a DHT key", i+1, fields[2])
		}
		p := tox.Port(port)
		nodes = append(nodes, node{fields[0], p, []tox.Port{p}, fields[2]})
	}
	return nodes, nil
}
