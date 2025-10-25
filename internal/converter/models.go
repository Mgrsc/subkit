package converter

type ProxyNode struct {
	Name                 string                 `yaml:"name"`
	Type                 string                 `yaml:"type"`
	Server               string                 `yaml:"server"`
	Port                 int                    `yaml:"port"`
	UUID                 string                 `yaml:"uuid,omitempty"`
	Password             string                 `yaml:"password,omitempty"`
	Cipher               string                 `yaml:"cipher,omitempty"`
	AlterID              int                    `yaml:"alterId,omitempty"`
	Network              string                 `yaml:"network,omitempty"`
	TLS                  bool                   `yaml:"tls,omitempty"`
	SNI                  string                 `yaml:"sni,omitempty"`
	Servername           string                 `yaml:"servername,omitempty"`
	Flow                 string                 `yaml:"flow,omitempty"`
	Encryption           string                 `yaml:"encryption,omitempty"`
	ClientFingerprint    string                 `yaml:"client-fingerprint,omitempty"`
	Plugin               string                 `yaml:"plugin,omitempty"`
	PluginOpts           map[string]interface{} `yaml:"plugin-opts,omitempty"`
	Protocol             string                 `yaml:"protocol,omitempty"`
	Obfs                 string                 `yaml:"obfs,omitempty"`
	ObfsParam            string                 `yaml:"obfs-param,omitempty"`
	ProtocolParam        string                 `yaml:"protocol-param,omitempty"`
	AuthStr              string                 `yaml:"auth-str,omitempty"`
	Up                   string                 `yaml:"up,omitempty"`
	Down                 string                 `yaml:"down,omitempty"`
	ObfsPassword         string                 `yaml:"obfs-password,omitempty"`
	SkipCertVerify       bool                   `yaml:"skip-cert-verify,omitempty"`
	ALPN                 []string               `yaml:"alpn,omitempty"`
	WsOpts               *WsOpts                `yaml:"ws-opts,omitempty"`
	GrpcOpts             *GrpcOpts              `yaml:"grpc-opts,omitempty"`
	RealityOpts          *RealityOpts           `yaml:"reality-opts,omitempty"`
	Token                string                 `yaml:"token,omitempty"`
	DisableSNI           bool                   `yaml:"disable-sni,omitempty"`
	ReduceRTT            bool                   `yaml:"reduce-rtt,omitempty"`
	UDPRelayMode         string                 `yaml:"udp-relay-mode,omitempty"`
	CongestionController string                 `yaml:"congestion-controller,omitempty"`
	Ports                string                 `yaml:"ports,omitempty"`
}

type WsOpts struct {
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers,omitempty"`
}

type GrpcOpts struct {
	GrpcServiceName string `yaml:"grpc-service-name"`
}

type RealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id"`
}

type MihomoConfig struct {
	Proxies       []ProxyNode    `yaml:"proxies,omitempty"`
	ProxyGroups   []ProxyGroup   `yaml:"proxy-groups,omitempty"`
	Rules         []string       `yaml:"rules,omitempty"`
	RuleProviders map[string]any `yaml:"rule-providers,omitempty"`
}

type ProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
	Strategy string   `yaml:"strategy,omitempty"`
}
