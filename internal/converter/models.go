package converter

type ProxyNode struct {
	Name                 string                 `yaml:"name" json:"name"`
	Type                 string                 `yaml:"type" json:"type"`
	Server               string                 `yaml:"server" json:"server"`
	Port                 int                    `yaml:"port" json:"port"`
	UUID                 string                 `yaml:"uuid,omitempty" json:"uuid,omitempty"`
	Password             string                 `yaml:"password,omitempty" json:"password,omitempty"`
	Cipher               string                 `yaml:"cipher,omitempty" json:"cipher,omitempty"`
	AlterID              int                    `yaml:"alterId,omitempty" json:"alterId,omitempty"`
	Network              string                 `yaml:"network,omitempty" json:"network,omitempty"`
	TLS                  bool                   `yaml:"tls,omitempty" json:"tls,omitempty"`
	SNI                  string                 `yaml:"sni,omitempty" json:"sni,omitempty"`
	Servername           string                 `yaml:"servername,omitempty" json:"servername,omitempty"`
	Flow                 string                 `yaml:"flow,omitempty" json:"flow,omitempty"`
	Encryption           string                 `yaml:"encryption,omitempty" json:"encryption,omitempty"`
	ClientFingerprint    string                 `yaml:"client-fingerprint,omitempty" json:"client-fingerprint,omitempty"`
	Plugin               string                 `yaml:"plugin,omitempty" json:"plugin,omitempty"`
	PluginOpts           map[string]interface{} `yaml:"plugin-opts,omitempty" json:"plugin-opts,omitempty"`
	Protocol             string                 `yaml:"protocol,omitempty" json:"protocol,omitempty"`
	Obfs                 string                 `yaml:"obfs,omitempty" json:"obfs,omitempty"`
	ObfsParam            string                 `yaml:"obfs-param,omitempty" json:"obfs-param,omitempty"`
	ProtocolParam        string                 `yaml:"protocol-param,omitempty" json:"protocol-param,omitempty"`
	AuthStr              string                 `yaml:"auth-str,omitempty" json:"auth-str,omitempty"`
	Up                   string                 `yaml:"up,omitempty" json:"up,omitempty"`
	Down                 string                 `yaml:"down,omitempty" json:"down,omitempty"`
	ObfsPassword         string                 `yaml:"obfs-password,omitempty" json:"obfs-password,omitempty"`
	SkipCertVerify       bool                   `yaml:"skip-cert-verify,omitempty" json:"skip-cert-verify,omitempty"`
	ALPN                 []string               `yaml:"alpn,omitempty" json:"alpn,omitempty"`
	WsOpts               *WsOpts                `yaml:"ws-opts,omitempty" json:"ws-opts,omitempty"`
	GrpcOpts             *GrpcOpts              `yaml:"grpc-opts,omitempty" json:"grpc-opts,omitempty"`
	RealityOpts          *RealityOpts           `yaml:"reality-opts,omitempty" json:"reality-opts,omitempty"`
	Token                string                 `yaml:"token,omitempty" json:"token,omitempty"`
	DisableSNI           bool                   `yaml:"disable-sni,omitempty" json:"disable-sni,omitempty"`
	ReduceRTT            bool                   `yaml:"reduce-rtt,omitempty" json:"reduce-rtt,omitempty"`
	UDPRelayMode         string                 `yaml:"udp-relay-mode,omitempty" json:"udp-relay-mode,omitempty"`
	CongestionController string                 `yaml:"congestion-controller,omitempty" json:"congestion-controller,omitempty"`
	Ports                string                 `yaml:"ports,omitempty" json:"ports,omitempty"`
}

type WsOpts struct {
	Path    string            `yaml:"path" json:"path"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

type GrpcOpts struct {
	GrpcServiceName string `yaml:"grpc-service-name" json:"grpc-service-name"`
}

type RealityOpts struct {
	PublicKey string `yaml:"public-key" json:"public-key"`
	ShortID   string `yaml:"short-id" json:"short-id"`
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
