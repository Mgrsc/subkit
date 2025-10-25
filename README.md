# Subkit

Lightweight, high-performance proxy subscription converter for Mihomo/Clash with LLM-powered configuration generation.

## Features

- **Multi-Protocol Support**: Vless, Vmess, Trojan, SS, SSR, Hysteria, Hysteria2, TUIC
- **LLM-Powered Generation**: Intelligent proxy-groups and rules configuration
- **Custom Requirements**: Tell the AI what specific rules you need (ad blocking, streaming, messaging apps)
- **Customizable Prompts**: Personalize LLM behavior for your needs
- **Rate Limiting**: Daily request limits to control usage
- **Auto-Update Rules**: GeoIP and GeoSite rules refresh every 2 days
- **Web Interface**: Easy-to-use conversion interface
- **RESTful API**: Integrate with your workflow
- **Docker Ready**: Multi-platform container support (amd64/arm64)

## Quick Start

### Docker (Recommended)

```bash
echo "LLM_API_KEY=your_api_key_here" > .env
docker compose up -d
```

Access the web interface at `http://localhost:8080`

### From Source

```bash
git clone https://github.com/mgrsc/subkit
cd subkit
cp .env.example .env
# Edit .env with your LLM API key
go mod tidy
go build -o subkit cmd/server/main.go
./subkit
```

## Configuration

Create `.env` file with your LLM credentials:

```env
LLM_API_KEY=your_api_key_here
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL=gpt-5-mini
LLM_TIMEOUT=120s
PORT=8080
DAILY_REQUEST_LIMIT=100
LOG_LEVEL=INFO
SUBKIT_SUBSCRIBE_NAME=Subkit Mihomo
```

### Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_API_KEY` | (required) | Your LLM API key |
| `LLM_BASE_URL` | `https://api.openai.com/v1` | LLM API endpoint |
| `LLM_MODEL` | `gpt-5-mini` | Model to use for generation |
| `LLM_TIMEOUT` | `120s` | Request timeout (supports 30s, 2m, 5m, etc.) |
| `PORT` | `8080` | Server port |
| `DAILY_REQUEST_LIMIT` | `100` | Maximum requests per day |
| `LOG_LEVEL` | `INFO` | Log level (DEBUG, INFO, WARN, ERROR) |
| `SUBKIT_SUBSCRIBE_NAME` | `Subkit Mihomo` | Subscription display name appended as `?name=` |

### Log Levels

- **DEBUG**: Detailed information including LLM request/response content
- **INFO**: General informational messages (default)
- **WARN**: Warning messages for non-critical issues
- **ERROR**: Error messages for failures

Set `LOG_LEVEL=DEBUG` to see detailed LLM I/O for debugging purposes.

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/convert` | POST | Convert subscription to Mihomo config |
| `/subscribe/{id}` | GET | Download generated configuration |
| `/api/node-to-uri` | POST | Convert Mihomo node to proxy URI |
| `/api/uri-to-node` | POST | Convert proxy URI to Mihomo node |

**Example Request:**
```bash
curl -X POST http://localhost:8080/api/convert \
  -H "Content-Type: application/json" \
  -d '{
    "input": "https://your-subscription-url",
    "custom_requirements": "Add ad blocking rules and Netflix streaming rules"
  }'
```

**Request Body:**
```json
{
  "input": "subscription_url_or_content",
  "uris": ["vless://...", "vmess://..."],
  "custom_requirements": "Optional: tell AI what specific rules you need"
}
```

**Response Headers:**
- `X-RateLimit-Limit`: Total daily request limit
- `X-RateLimit-Remaining`: Remaining requests today
- `X-RateLimit-Reset`: Timestamp when limit resets

## Advanced Features

### Custom Requirements

Tell the AI what specific rules you need when generating configurations:

**Web Interface:**
- Enter your custom requirements in the "Custom Requirements" field
- Examples:
  - "Add ad blocking rules"
  - "Include Netflix and Disney+ streaming rules"
  - "Add Telegram and WhatsApp messaging rules"
  - "Block trackers and analytics"

**API:**
```json
{
  "input": "your_subscription_url",
  "custom_requirements": "Add comprehensive ad blocking and streaming optimization"
}
```

The AI will automatically:
- Create appropriate rule-providers
- Add routing rules
- Optimize proxy group selection

### Custom LLM Prompts

Customize configuration generation by editing prompt files in `config/prompts/`:

```
config/prompts/
├── proxy_groups_system.txt  # System prompt for proxy groups (role & rules)
├── proxy_groups_user.txt    # User prompt template with variables
├── rules_system.txt         # System prompt for rules (role & rules)
└── rules_user.txt           # User prompt template with variables
```

**Important Output Format Requirements:**

When customizing prompts, the LLM must output specific YAML structures:

1. **Proxy Groups** (`proxy_groups_system.txt`):
   - Must start with `proxy-groups:` on the first line
   - Use 2-space indentation
   - Valid YAML only, no markdown code blocks or explanations

2. **Rules** (`rules_system.txt`):
   - Must include both `rule-providers:` and `rules:` sections
   - Valid YAML only, no markdown code blocks or explanations

**Prompt Structure:**
- `*_system.txt`: System role, instructions, and examples (no variables)
- `*_user.txt`: Variable placeholders using XML tags (e.g., `<可用代理>{PROXIES}</可用代理>`)

**Available Variables:**
- Proxy Groups: `{PROXIES}`, `{CUSTOM_REQUIREMENTS}`
- Rules: `{PROXY_GROUPS}`, `{GEOIP_FILES}`, `{GEOSITE_FILES}`, `{CUSTOM_REQUIREMENTS}`

**Example Output (proxy_groups):**
```yaml
proxy-groups:
  - name: 🚀 International node
    type: select
    proxies:
      - ⚡ Automatic selection
      - DIRECT
```

**Example Output (rules):**
```yaml
rule-providers:
  ads:
    type: http
    behavior: domain
    url: "https://example.com/ads.yaml"
    interval: 86400

rules:
  - RULE-SET,ads,🛑 Ad blocking
  - MATCH,🐟 omission
```

### Manual Rule Updates

```bash
# Build update tool
go build -o update-rules cmd/update-rules/main.go

# Update rules
./update-rules
```

Or in Docker:
```bash
docker compose exec subkit /app/update-rules
```

Rules are automatically updated every 2 days when the server is running.

## Docker Deployment

### Pull from GitHub Container Registry

```bash
docker pull ghcr.io/mgrsc/subkit:latest
```

### Available Tags

- `latest` - Latest stable release
- `v1.0.0` - Specific version
- `main-abc1234-20250125` - Snapshot builds

## Project Structure

```
subkit/
├── cmd/
│   ├── server/         # Main application
│   └── update-rules/   # Rule updater tool
├── internal/
│   ├── converter/      # Protocol parsing
│   ├── llm/           # LLM client
│   ├── config/        # Config assembler
│   ├── scheduler/     # Auto-updater
│   └── server/        # HTTP handlers
├── web/static/        # Web interface
├── config/
│   ├── prompts/       # LLM prompts (system & user)
│   ├── rules/         # Rule lists
│   └── global.yaml    # Global config
└── .github/workflows/ # CI/CD
```

## Development

### Requirements

- Go 1.25.1+
- OpenAI-compatible LLM API
- Docker & Docker Compose (optional)

### Adding New Protocols

1. Extend parser in `internal/converter/parser.go`
2. Update models in `internal/converter/models.go`
3. Add tests
4. Submit PR

### Building

```bash
# Server
go build -o subkit cmd/server/main.go

# Update tool
go build -o update-rules cmd/update-rules/main.go

# Both with optimizations
CGO_ENABLED=0 go build -ldflags="-s -w" -o subkit cmd/server/main.go
CGO_ENABLED=0 go build -ldflags="-s -w" -o update-rules cmd/update-rules/main.go
```

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## License

MIT License - see [MIT License](LICENSE) for details

## Support

- **Issues**: [GitHub Issues](https://github.com/mgrsc/subkit/issues)
- **Discussions**: [GitHub Discussions](https://github.com/mgrsc/subkit/discussions)
