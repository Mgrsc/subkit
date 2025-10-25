package converter

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func UriToProxy(uri string) (*ProxyNode, error) {
	if !strings.Contains(uri, "://") {
		return nil, fmt.Errorf("invalid uri format")
	}

	scheme := strings.Split(uri, "://")[0]
	scheme = strings.ToLower(scheme)

	if scheme == "hy2" {
		uri = "hysteria2" + uri[3:]
		scheme = "hysteria2"
	}

	switch scheme {
	case "ss":
		return parseSS(uri)
	case "ssr":
		return parseSSR(uri)
	case "vmess":
		return parseVmess(uri)
	case "vless":
		return parseVless(uri)
	case "trojan":
		return parseTrojan(uri)
	case "hysteria":
		return parseHysteria(uri)
	case "hysteria2":
		return parseHysteria2(uri)
	case "tuic":
		return parseTuic(uri)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", scheme)
	}
}

func ProxyToUri(node *ProxyNode) (string, error) {
	switch strings.ToLower(node.Type) {
	case "ss":
		return emitSS(node)
	case "ssr":
		return emitSSR(node)
	case "vmess":
		return emitVmess(node)
	case "vless":
		return emitVless(node)
	case "trojan":
		return emitTrojan(node)
	case "hysteria":
		return emitHysteria(node)
	case "hysteria2":
		return emitHysteria2(node)
	case "tuic":
		return emitTuic(node)
	default:
		return "", fmt.Errorf("unsupported node type: %s", node.Type)
	}
}

func parseSS(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	name := u.Fragment
	if name == "" {
		name = "ss"
	}

	if u.Host != "" {
		userinfo := u.User.Username()
		decoded, err := b64decode(userinfo)
		if err != nil {
			return nil, err
		}

		parts := strings.SplitN(decoded, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid ss userinfo")
		}

		node := &ProxyNode{
			Name:     name,
			Type:     "ss",
			Server:   u.Hostname(),
			Port:     parsePort(u.Port()),
			Cipher:   parts[0],
			Password: parts[1],
		}

		plugin := u.Query().Get("plugin")
		if plugin != "" {
			pluginParts := strings.SplitN(plugin, ";", 2)
			node.Plugin = pluginParts[0]
			if len(pluginParts) == 2 {
				opts := make(map[string]interface{})
				for _, kv := range strings.Split(pluginParts[1], ";") {
					if kv == "" {
						continue
					}
					kvp := strings.SplitN(kv, "=", 2)
					if len(kvp) == 2 {
						opts[kvp[0]] = kvp[1]
					}
				}
				if len(opts) > 0 {
					node.PluginOpts = opts
				}
			}
		}

		return node, nil
	}

	content, err := b64decode(strings.TrimPrefix(u.Path, "/"))
	if err != nil {
		return nil, err
	}

	re := regexp.MustCompile(`^([^:@]+):([^@]+)@([^:]+):(\d+)$`)
	matches := re.FindStringSubmatch(content)
	if len(matches) != 5 {
		return nil, fmt.Errorf("invalid ss format")
	}

	port, _ := strconv.Atoi(matches[4])
	return &ProxyNode{
		Name:     name,
		Type:     "ss",
		Server:   matches[3],
		Port:     port,
		Cipher:   matches[1],
		Password: matches[2],
	}, nil
}

func parseSSR(uri string) (*ProxyNode, error) {
	content, err := b64decode(strings.TrimPrefix(uri, "ssr://"))
	if err != nil {
		return nil, err
	}

	mainPart := content
	queryPart := ""
	if idx := strings.Index(content, "/?"); idx >= 0 {
		mainPart = content[:idx]
		queryPart = content[idx+2:]
	}

	parts := strings.Split(mainPart, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid ssr format")
	}

	password, _ := b64decode(parts[5])
	port, _ := strconv.Atoi(parts[1])

	node := &ProxyNode{
		Name:     "ssr",
		Type:     "ssr",
		Server:   parts[0],
		Port:     port,
		Protocol: parts[2],
		Cipher:   parts[3],
		Obfs:     parts[4],
		Password: password,
	}

	if queryPart != "" {
		q, _ := url.ParseQuery(queryPart)
		if obfsParam := q.Get("obfsparam"); obfsParam != "" {
			node.ObfsParam, _ = b64decode(obfsParam)
		}
		if protoParam := q.Get("protoparam"); protoParam != "" {
			node.ProtocolParam, _ = b64decode(protoParam)
		}
	}

	return node, nil
}

func parseVmess(uri string) (*ProxyNode, error) {
	payload, err := b64decode(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, err
	}

	node := &ProxyNode{
		Name:     getString(data, "ps", "vmess"),
		Type:     "vmess",
		Server:   getString(data, "add", ""),
		Port:     getInt(data, "port", 0),
		UUID:     getString(data, "id", ""),
		AlterID:  getInt(data, "aid", 0),
		Cipher:   getString(data, "scy", "auto"),
		Network:  getString(data, "net", "tcp"),
	}

	if getString(data, "tls", "") == "tls" {
		node.TLS = true
		if sni := getString(data, "sni", ""); sni != "" {
			node.Servername = sni
		}
	}

	if node.Network == "ws" {
		node.WsOpts = &WsOpts{
			Path: getString(data, "path", "/"),
		}
		if host := getString(data, "host", ""); host != "" {
			node.WsOpts.Headers = map[string]string{"Host": host}
		}
	} else if node.Network == "grpc" {
		node.GrpcOpts = &GrpcOpts{
			GrpcServiceName: getString(data, "path", ""),
		}
	}

	return node, nil
}

func parseVless(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	node := &ProxyNode{
		Name:     u.Fragment,
		Type:     "vless",
		Server:   u.Hostname(),
		Port:     parsePort(u.Port()),
		UUID:     u.User.Username(),
		Network:  q.Get("type"),
	}

	if node.Name == "" {
		node.Name = "vless"
	}
	if node.Network == "" {
		node.Network = "tcp"
	}

	if enc := q.Get("encryption"); enc != "" {
		node.Encryption = enc
	}
	if flow := q.Get("flow"); flow != "" {
		node.Flow = flow
	}

	security := q.Get("security")
	if security == "reality" {
		node.TLS = true
		node.RealityOpts = &RealityOpts{
			PublicKey: q.Get("pbk"),
			ShortID:   q.Get("sid"),
		}
		if sni := q.Get("sni"); sni != "" {
			node.Servername = sni
		}
		if fp := q.Get("fp"); fp != "" {
			node.ClientFingerprint = fp
		}
	} else if security == "tls" {
		node.TLS = true
		if sni := q.Get("sni"); sni != "" {
			node.Servername = sni
		}
		if alpn := q.Get("alpn"); alpn != "" {
			node.ALPN = strings.Split(alpn, ",")
		}
		if fp := q.Get("fp"); fp != "" {
			node.ClientFingerprint = fp
		}
	}

	if node.Network == "ws" {
		node.WsOpts = &WsOpts{
			Path: q.Get("path"),
		}
		if node.WsOpts.Path == "" {
			node.WsOpts.Path = "/"
		}
		if host := q.Get("host"); host != "" {
			node.WsOpts.Headers = map[string]string{"Host": host}
		}
	} else if node.Network == "grpc" {
		node.GrpcOpts = &GrpcOpts{
			GrpcServiceName: q.Get("serviceName"),
		}
	}

	return node, nil
}

func parseTrojan(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	node := &ProxyNode{
		Name:     u.Fragment,
		Type:     "trojan",
		Server:   u.Hostname(),
		Port:     parsePort(u.Port()),
		Password: u.User.Username(),
		Network:  q.Get("type"),
	}

	if node.Name == "" {
		node.Name = "trojan"
	}
	if node.Network == "" {
		node.Network = "tcp"
	}

	security := q.Get("security")
	if security == "reality" {
		node.TLS = true
		node.RealityOpts = &RealityOpts{
			PublicKey: q.Get("pbk"),
			ShortID:   q.Get("sid"),
		}
		if sni := q.Get("sni"); sni != "" {
			node.SNI = sni
		} else if peer := q.Get("peer"); peer != "" {
			node.SNI = peer
		}
		if fp := q.Get("fp"); fp != "" {
			node.ClientFingerprint = fp
		}
	} else {
		if q.Get("security") == "" || q.Get("security") == "tls" {
			node.TLS = true
		}
		if sni := q.Get("sni"); sni != "" {
			node.SNI = sni
		} else if peer := q.Get("peer"); peer != "" {
			node.SNI = peer
		}
		if alpn := q.Get("alpn"); alpn != "" {
			node.ALPN = strings.Split(alpn, ",")
		}
		if fp := q.Get("fp"); fp != "" {
			node.ClientFingerprint = fp
		}
	}

	return node, nil
}

func parseHysteria(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	node := &ProxyNode{
		Name:     u.Fragment,
		Type:     "hysteria",
		Server:   u.Hostname(),
		Port:     parsePort(u.Port()),
		AuthStr:  u.User.Username(),
		Protocol: q.Get("protocol"),
		Up:       q.Get("up"),
		Down:     q.Get("down"),
	}

	if node.Name == "" {
		node.Name = "hysteria"
	}
	if node.Protocol == "" {
		node.Protocol = "udp"
	}

	if sni := q.Get("sni"); sni != "" {
		node.SNI = sni
	}
	if q.Get("insecure") == "1" {
		node.SkipCertVerify = true
	}
	if obfs := q.Get("obfs"); obfs != "" {
		node.Obfs = obfs
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}

	return node, nil
}

func parseHysteria2(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	node := &ProxyNode{
		Name:     u.Fragment,
		Type:     "hysteria2",
		Server:   u.Hostname(),
		Port:     parsePort(u.Port()),
		Password: u.User.Username(),
	}

	if node.Name == "" {
		node.Name = "hysteria2"
	}

	if up := q.Get("up"); up != "" {
		node.Up = up
	} else if up := q.Get("upmbps"); up != "" {
		node.Up = up
	}

	if down := q.Get("down"); down != "" {
		node.Down = down
	} else if down := q.Get("downmbps"); down != "" {
		node.Down = down
	}

	if sni := q.Get("sni"); sni != "" {
		node.SNI = sni
	}
	if q.Get("insecure") == "1" {
		node.SkipCertVerify = true
	}
	if obfs := q.Get("obfs"); obfs != "" {
		node.Obfs = obfs
	}
	if obfsPass := q.Get("obfs-password"); obfsPass != "" {
		node.ObfsPassword = obfsPass
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}
	if ports := q.Get("ports"); ports != "" {
		node.Ports = ports
	}

	return node, nil
}

func parseTuic(uri string) (*ProxyNode, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	user := u.User.Username()
	pass, _ := u.User.Password()

	node := &ProxyNode{
		Name:   u.Fragment,
		Type:   "tuic",
		Server: u.Hostname(),
		Port:   parsePort(u.Port()),
	}

	if node.Name == "" {
		node.Name = "tuic"
	}

	uuidPattern := regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)
	if uuidPattern.MatchString(user) {
		node.UUID = user
		if pass != "" {
			node.Password = pass
		}
	} else {
		if user != "" {
			node.Token = user
		}
		if pass != "" {
			node.Password = pass
		}
	}

	if token := q.Get("token"); token != "" {
		node.Token = token
	}
	if sni := q.Get("sni"); sni != "" {
		node.SNI = sni
	}
	if q.Get("skip-cert-verify") == "1" {
		node.SkipCertVerify = true
	}
	if alpn := q.Get("alpn"); alpn != "" {
		node.ALPN = strings.Split(alpn, ",")
	}
	if q.Get("disable-sni") == "1" {
		node.DisableSNI = true
	}
	if q.Get("reduce-rtt") == "1" {
		node.ReduceRTT = true
	}
	if mode := q.Get("udp-relay-mode"); mode != "" {
		node.UDPRelayMode = mode
	}
	if cc := q.Get("congestion-controller"); cc != "" {
		node.CongestionController = cc
	}

	return node, nil
}

func emitSS(n *ProxyNode) (string, error) {
	userinfo := b64encode(n.Cipher + ":" + n.Password)
	query := url.Values{}

	if n.Plugin != "" {
		pluginStr := n.Plugin
		if n.PluginOpts != nil {
			parts := []string{n.Plugin}
			if n.Plugin == "obfs" {
				if mode, ok := n.PluginOpts["mode"].(string); ok {
					parts = append(parts, "obfs="+mode)
				}
				if host, ok := n.PluginOpts["host"].(string); ok {
					parts = append(parts, "obfs-host="+host)
				} else if host, ok := n.PluginOpts["obfs-host"].(string); ok {
					parts = append(parts, "obfs-host="+host)
				}
			} else {
				for k, v := range n.PluginOpts {
					parts = append(parts, fmt.Sprintf("%s=%v", k, v))
				}
			}
			pluginStr = strings.Join(parts, ";")
		}
		query.Set("plugin", pluginStr)
	}

	queryStr := ""
	if len(query) > 0 {
		queryStr = "?" + query.Encode()
	}

	name := n.Name
	if name == "" {
		name = "ss"
	}

	return fmt.Sprintf("ss://%s@%s:%d%s#%s", userinfo, n.Server, n.Port, queryStr, url.QueryEscape(name)), nil
}

func emitSSR(n *ProxyNode) (string, error) {
	protocol := n.Protocol
	if protocol == "" {
		protocol = "origin"
	}
	cipher := n.Cipher
	if cipher == "" {
		cipher = "aes-128-ctr"
	}
	obfs := n.Obfs
	if obfs == "" {
		obfs = "plain"
	}

	pwdB64 := b64encode(n.Password)
	main := fmt.Sprintf("%s:%d:%s:%s:%s:%s", n.Server, n.Port, protocol, cipher, obfs, pwdB64)

	query := url.Values{}
	if n.ObfsParam != "" {
		query.Set("obfsparam", b64encode(n.ObfsParam))
	}
	if n.ProtocolParam != "" {
		query.Set("protoparam", b64encode(n.ProtocolParam))
	}

	queryStr := ""
	if len(query) > 0 {
		queryStr = "/?" + query.Encode()
	}

	return "ssr://" + b64encode(main+queryStr), nil
}

func emitVmess(n *ProxyNode) (string, error) {
	data := map[string]interface{}{
		"v":    "2",
		"ps":   n.Name,
		"add":  n.Server,
		"port": n.Port,
		"id":   n.UUID,
		"aid":  n.AlterID,
		"scy":  n.Cipher,
		"net":  n.Network,
		"type": "none",
		"host": "",
		"path": "",
		"tls":  "",
		"sni":  "",
	}

	if n.Cipher == "" {
		data["scy"] = "auto"
	}
	if n.Network == "" {
		data["net"] = "tcp"
	}

	if n.TLS {
		data["tls"] = "tls"
		if n.Servername != "" {
			data["sni"] = n.Servername
		} else if n.SNI != "" {
			data["sni"] = n.SNI
		}
	}

	if n.Network == "ws" && n.WsOpts != nil {
		data["path"] = n.WsOpts.Path
		if n.WsOpts.Headers != nil {
			if host, ok := n.WsOpts.Headers["Host"]; ok {
				data["host"] = host
			}
		}
	} else if n.Network == "grpc" && n.GrpcOpts != nil {
		data["path"] = n.GrpcOpts.GrpcServiceName
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return "vmess://" + b64encode(string(jsonBytes)), nil
}

func emitVless(n *ProxyNode) (string, error) {
	network := n.Network
	if network == "" {
		network = "tcp"
	}

	query := url.Values{}
	query.Set("type", network)

	if n.Encryption != "" {
		query.Set("encryption", n.Encryption)
	}
	if n.Flow != "" {
		query.Set("flow", n.Flow)
	}

	if n.TLS {
		if n.RealityOpts != nil {
			query.Set("security", "reality")
			query.Set("pbk", n.RealityOpts.PublicKey)
			query.Set("sid", n.RealityOpts.ShortID)
			if n.Servername != "" {
				query.Set("sni", n.Servername)
			} else if n.SNI != "" {
				query.Set("sni", n.SNI)
			}
			if n.ClientFingerprint != "" {
				query.Set("fp", n.ClientFingerprint)
			}
		} else {
			query.Set("security", "tls")
			if n.Servername != "" {
				query.Set("sni", n.Servername)
			} else if n.SNI != "" {
				query.Set("sni", n.SNI)
			}
			if len(n.ALPN) > 0 {
				query.Set("alpn", strings.Join(n.ALPN, ","))
			}
			if n.ClientFingerprint != "" {
				query.Set("fp", n.ClientFingerprint)
			}
		}
	}

	if network == "ws" && n.WsOpts != nil {
		path := n.WsOpts.Path
		if path == "" {
			path = "/"
		}
		query.Set("path", path)
		if n.WsOpts.Headers != nil {
			if host, ok := n.WsOpts.Headers["Host"]; ok {
				query.Set("host", host)
			}
		}
	} else if network == "grpc" && n.GrpcOpts != nil {
		query.Set("serviceName", n.GrpcOpts.GrpcServiceName)
	}

	name := n.Name
	if name == "" {
		name = "vless"
	}

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s",
		n.UUID, n.Server, n.Port, query.Encode(), url.QueryEscape(name)), nil
}

func emitTrojan(n *ProxyNode) (string, error) {
	network := n.Network
	if network == "" {
		network = "tcp"
	}

	query := url.Values{}
	query.Set("type", network)

	if n.TLS || n.RealityOpts != nil {
		if n.RealityOpts != nil {
			query.Set("security", "reality")
			query.Set("pbk", n.RealityOpts.PublicKey)
			query.Set("sid", n.RealityOpts.ShortID)
		} else {
			query.Set("security", "tls")
		}
	}

	if n.SNI != "" {
		query.Set("sni", n.SNI)
	} else if n.Servername != "" {
		query.Set("sni", n.Servername)
	}

	if len(n.ALPN) > 0 {
		query.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.ClientFingerprint != "" {
		query.Set("fp", n.ClientFingerprint)
	}

	name := n.Name
	if name == "" {
		name = "trojan"
	}

	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s",
		n.Password, n.Server, n.Port, query.Encode(), url.QueryEscape(name)), nil
}

func emitHysteria(n *ProxyNode) (string, error) {
	query := url.Values{}

	protocol := n.Protocol
	if protocol == "" {
		protocol = "udp"
	}
	query.Set("protocol", protocol)

	if n.Up != "" {
		query.Set("up", n.Up)
	}
	if n.Down != "" {
		query.Set("down", n.Down)
	}
	if n.SNI != "" {
		query.Set("sni", n.SNI)
	}
	if n.Obfs != "" {
		query.Set("obfs", n.Obfs)
	}
	if len(n.ALPN) > 0 {
		query.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.SkipCertVerify {
		query.Set("insecure", "1")
	}

	name := n.Name
	if name == "" {
		name = "hysteria"
	}

	auth := ""
	if n.AuthStr != "" {
		auth = n.AuthStr + "@"
	}

	return fmt.Sprintf("hysteria://%s%s:%d?%s#%s",
		auth, n.Server, n.Port, query.Encode(), url.QueryEscape(name)), nil
}

func emitHysteria2(n *ProxyNode) (string, error) {
	query := url.Values{}

	if n.Up != "" {
		query.Set("up", n.Up)
	}
	if n.Down != "" {
		query.Set("down", n.Down)
	}
	if n.SNI != "" {
		query.Set("sni", n.SNI)
	}
	if n.Obfs != "" {
		query.Set("obfs", n.Obfs)
	}
	if n.ObfsPassword != "" {
		query.Set("obfs-password", n.ObfsPassword)
	}
	if len(n.ALPN) > 0 {
		query.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.Ports != "" {
		query.Set("ports", n.Ports)
	}
	if n.SkipCertVerify {
		query.Set("insecure", "1")
	}

	name := n.Name
	if name == "" {
		name = "hysteria2"
	}

	auth := ""
	if n.Password != "" {
		auth = n.Password + "@"
	}

	return fmt.Sprintf("hysteria2://%s%s:%d?%s#%s",
		auth, n.Server, n.Port, query.Encode(), url.QueryEscape(name)), nil
}

func emitTuic(n *ProxyNode) (string, error) {
	query := url.Values{}

	if n.Token != "" {
		query.Set("token", n.Token)
	}
	if n.SNI != "" {
		query.Set("sni", n.SNI)
	}
	if n.SkipCertVerify {
		query.Set("skip-cert-verify", "1")
	}
	if len(n.ALPN) > 0 {
		query.Set("alpn", strings.Join(n.ALPN, ","))
	}
	if n.DisableSNI {
		query.Set("disable-sni", "1")
	}
	if n.ReduceRTT {
		query.Set("reduce-rtt", "1")
	}
	if n.UDPRelayMode != "" {
		query.Set("udp-relay-mode", n.UDPRelayMode)
	}
	if n.CongestionController != "" {
		query.Set("congestion-controller", n.CongestionController)
	}

	name := n.Name
	if name == "" {
		name = "tuic"
	}

	auth := ""
	if n.UUID != "" || n.Password != "" {
		if n.Password != "" {
			auth = n.UUID + ":" + n.Password + "@"
		} else {
			auth = n.UUID + "@"
		}
	}

	return fmt.Sprintf("tuic://%s%s:%d?%s#%s",
		auth, n.Server, n.Port, query.Encode(), url.QueryEscape(name)), nil
}

func b64encode(s string) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(s))
}

func b64decode(s string) (string, error) {
	pad := (-len(s)) % 4
	s += strings.Repeat("=", pad)
	data, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parsePort(s string) int {
	port, _ := strconv.Atoi(s)
	return port
}

func getString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func getInt(m map[string]interface{}, key string, def int) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return int(val)
		case int:
			return val
		case string:
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
		}
	}
	return def
}
