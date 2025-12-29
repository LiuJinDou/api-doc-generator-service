# API Documentation Generator Service

A standalone microservice that automatically analyzes code changes from Git commits, generates OpenAPI documentation, and synchronizes it to Apifox.

## Features

- 🔄 **Automatic Code Analysis**: Parses code changes from Git webhooks
- 📝 **OpenAPI Generation**: Auto-generates OpenAPI 3.0 specifications from code
- 🔌 **Pluggable Parsers**: Extensible architecture for multiple languages/frameworks
- 🚀 **Apifox Integration**: Automatically syncs documentation to Apifox
- 🐳 **Docker Ready**: Easy deployment with Docker/Kubernetes
- 🎯 **Webhook Support**: GitHub and GitLab webhook integration

## Currently Supported

- ✅ **Go + Gin Framework**

## Planned Support

- 🔜 Node.js + Express
- 🔜 Python + FastAPI
- 🔜 Java + Spring Boot

## Architecture

```
Git Webhook → Service Listener → Code Analyzer → OpenAPI Generator → Apifox Sync
```

## Quick Start

### Prerequisites

- Go 1.21+ (for local development)
- Docker & Docker Compose (for containerized deployment)
- Git
- Apifox account with API token

### 1. Clone the Repository

```bash
git clone https://github.com/LiuJinDou/api-doc-generator-service.git
cd api-doc-generator-service
```

### 2. Configuration

Copy the example environment file and configure it:

```bash
cp .env.example .env
```

Edit `.env` with your settings:

```env
SERVER_PORT=8080
GIT_WORK_DIR=/tmp/repos
WEBHOOK_SECRET=your-webhook-secret
APIFOX_TOKEN=your-apifox-token
APIFOX_PROJECT_ID=your-project-id
```

**How to get Apifox credentials:**
1. Log in to Apifox
2. Go to Account Settings → API Tokens
3. Create a new token
4. Get your Project ID from the Apifox project URL

### 3. Run with Docker (Recommended)

```bash
# Build and start the service
docker-compose up -d

# View logs
docker-compose logs -f

# Stop the service
docker-compose down
```

### 4. Run Locally

```bash
# Install dependencies
go mod download

# Run the service
go run cmd/server/main.go

# Or build and run
make build
./api-doc-generator
```

## Usage

### GitHub Webhook Setup

1. Go to your repository → Settings → Webhooks
2. Click "Add webhook"
3. Configure:
   - **Payload URL**: `http://your-server:8080/webhook/github`
   - **Content type**: `application/json`
   - **Secret**: Your `WEBHOOK_SECRET` value
   - **Events**: Select "Just the push event"
4. Click "Add webhook"

### GitLab Webhook Setup

1. Go to your repository → Settings → Webhooks
2. Configure:
   - **URL**: `http://your-server:8080/webhook/gitlab`
   - **Secret Token**: Your `WEBHOOK_SECRET` value
   - **Trigger**: Check "Push events"
3. Click "Add webhook"

### Manual Trigger

You can also trigger analysis manually via API:

```bash
curl -X POST http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "repository_url": "https://github.com/yourusername/yourrepo.git",
    "branch": "main"
  }'
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/v1/info` | GET | Service information |
| `/webhook/github` | POST | GitHub webhook receiver |
| `/webhook/gitlab` | POST | GitLab webhook receiver |
| `/api/v1/analyze` | POST | Manual analysis trigger |

## How It Works

1. **Receive Webhook**: Service receives push event from GitHub/GitLab
2. **Clone Repository**: Clones or pulls latest code changes
3. **Detect Language**: Automatically detects project language/framework
4. **Parse Code**: Uses appropriate parser to analyze code structure
5. **Generate OpenAPI**: Creates OpenAPI 3.0 specification
6. **Sync to Apifox**: Uploads documentation to Apifox

## Development

### Project Structure

```
api-doc-generator-service/
├── cmd/server/              # Application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── webhook/            # Webhook handlers
│   ├── git/                # Git operations
│   ├── parser/             # Parser registry and implementations
│   │   └── gin/           # Gin framework parser
│   ├── openapi/           # OpenAPI spec builder
│   └── sync/              # Apifox synchronization
├── pkg/ast/               # AST analysis utilities
├── deployments/           # Deployment configurations
│   ├── docker/           # Docker files
│   └── k8s/              # Kubernetes manifests
└── configs/              # Configuration files
```

### Adding a New Parser

To add support for a new language/framework:

1. Create a new parser in `internal/parser/yourframework/`
2. Implement the `Parser` interface
3. Register it in `cmd/server/main.go`

Example:

```go
// internal/parser/express/express_parser.go
package express

import (
    "api-doc-generator/internal/openapi"
)

type ExpressParser struct{}

func NewExpressParser() *ExpressParser {
    return &ExpressParser{}
}

func (p *ExpressParser) Name() string {
    return "Express.js Parser"
}

func (p *ExpressParser) Language() string {
    return "node-express"
}

func (p *ExpressParser) Analyze(projectPath string) (*openapi.Spec, error) {
    // Implement parsing logic
    return spec, nil
}

// Register in main.go
parserRegistry.Register("node-express", express.NewExpressParser())
```

### Building

```bash
# Build binary
make build

# Run tests
make test

# Format code
make fmt

# Build Docker image
make docker-build
```

## Deployment

### Docker Deployment

```bash
docker-compose up -d
```

### Kubernetes Deployment

```bash
# Create secret
kubectl create secret generic api-doc-generator-secrets \
  --from-literal=apifox-token=YOUR_TOKEN \
  --from-literal=apifox-project-id=YOUR_PROJECT_ID

# Deploy
kubectl apply -f deployments/k8s/deployment.yaml

# Check status
kubectl get pods -l app=api-doc-generator
```

## Configuration Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `SERVER_PORT` | HTTP server port | `8080` |
| `GIT_WORK_DIR` | Directory for cloning repos | `/tmp/repos` |
| `WEBHOOK_SECRET` | Webhook signature validation secret | `` |
| `APIFOX_TOKEN` | Apifox API token | Required |
| `APIFOX_PROJECT_ID` | Apifox project ID | Required |
| `APIFOX_BASE_URL` | Apifox API base URL | `https://api.apifox.cn` |

## Troubleshooting

### Service not receiving webhooks

- Check firewall settings
- Ensure the service is publicly accessible
- Verify webhook secret matches

### Code analysis fails

- Check logs: `docker-compose logs -f`
- Verify the repository is accessible
- Ensure the language is supported

### Apifox sync fails

- Verify `APIFOX_TOKEN` and `APIFOX_PROJECT_ID`
- Check Apifox API status
- Review API response in logs

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- 📧 Issues: https://github.com/LiuJinDou/api-doc-generator-service/issues
- 📖 Documentation: See this README

## Roadmap

- [x] Go + Gin parser
- [ ] Node.js + Express parser
- [ ] Python + FastAPI parser
- [ ] Java + Spring Boot parser
- [ ] Database storage for history tracking
- [ ] API change diff detection
- [ ] Notification system
- [ ] Web UI dashboard
