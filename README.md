# RSS Feed Backend

A robust, production-ready RSS feed aggregation and processing backend built with Go. This service provides comprehensive APIs for fetching, storing, and managing RSS feed data with advanced security, monitoring, and performance optimization features.

## 🚀 Features

### Core Functionality
- **RSS Feed Processing**: Fetch and parse RSS feeds from any URL
- **Data Storage**: Store parsed feeds in Google Cloud Datastore
- **Feed Management**: Retrieve predefined or categorized RSS feed sources
- **Async Processing**: Background job processing for large feeds
- **Caching**: Multi-level caching with adaptive TTL based on feed frequency

### Security & Performance
- **Enhanced Rate Limiting**: Multi-factor client identification (IP, User-Agent, Accept-Language, session cookies)
- **Comprehensive URL Validation**: Protection against XSS, SSRF, and injection attacks
- **Configurable CORS**: Environment-specific CORS settings with subdomain support
- **Private Network Detection**: Blocks requests to internal/private networks
- **Input Sanitization**: Validates and sanitizes all user inputs

### Monitoring & Observability
- **Prometheus Metrics**: Comprehensive metrics for feed operations
- **OpenTelemetry Tracing**: Distributed tracing with Jaeger integration
- **Structured Logging**: JSON-based logging with logrus
- **Health Checks**: Liveness and readiness endpoints
- **Alert Management**: Configurable alerting for system events

### Performance Optimization
- **Adaptive Caching**: Different TTL strategies based on feed update frequency
- **Batch Processing**: Configurable batch sizes for different feed types
- **Async Queue**: Background processing with backpressure control
- **Connection Pooling**: Optimized database connections

## 📋 API Endpoints

### Feed Operations
- `POST /fetch-store` - Fetch and store RSS feed data (supports async processing)
- `GET /feeds` - Retrieve predefined RSS feed sources
- `GET /items` - Get feed items with pagination and filtering
- `GET /items/legacy` - Legacy endpoint for feed items
- `GET /job-status` - Check status of async processing jobs

### System Endpoints
- `GET /health` - Basic health check
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe
- `GET /metrics` - Prometheus metrics endpoint
- `GET /swagger/` - API documentation (Swagger UI)

## 🔧 Configuration

The application uses environment variables for configuration. Key configuration options:

### Database & Storage
```bash
PROJECT_ID=your-gcp-project-id
```

### Rate Limiting
```bash
RATE_LIMIT_RPM=10              # Requests per minute
RATE_LIMIT_BURST=5             # Burst capacity
CLIENT_CLEANUP_INTERVAL=1m     # Client cleanup interval
```

### CORS Configuration
```bash
ENVIRONMENT=development         # development, staging, production
DEV_CORS_ORIGINS=http://localhost:3000,http://127.0.0.1:3000
STAGING_CORS_ORIGINS=https://staging.yourdomain.com
PROD_CORS_ORIGINS=https://yourdomain.com
CORS_ALLOW_CREDENTIALS=true
CORS_ALLOW_SUBDOMAINS=false
```

### Performance Settings
```bash
DEFAULT_FEED_TTL=15m           # Default cache TTL for feeds
DEFAULT_ITEMS_TTL=30m          # Default cache TTL for items
HIGH_FREQ_FEED_TTL=5m          # TTL for frequently updated feeds
LOW_FREQ_FEED_TTL=60m          # TTL for rarely updated feeds

ASYNC_WORKERS=3                # Number of async workers
ASYNC_QUEUE_SIZE=50            # Async queue size
ASYNC_BACKPRESSURE=true        # Enable backpressure
ASYNC_REJECT_THRESHOLD=0.8     # Reject at 80% capacity
```

## 🚀 Getting Started

### Prerequisites
- Go 1.24.0 or higher
- Google Cloud Project with Datastore API enabled
- GCP credentials configured

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/Nexora-Open-Source/rss-feed-backend.git
   cd rss-feed-backend
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Configure environment variables**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Run the application**
   ```bash
   go run main.go
   ```

The server will start on `http://localhost:8080`

### Docker Deployment

```bash
# Build the image
docker build -t rss-feed-backend .

# Run the container
docker run -p 8080:8080 --env-file .env rss-feed-backend
```

### Google Cloud Deployment

```bash
# Deploy to App Engine
gcloud app deploy

# Deploy to Cloud Run
gcloud run deploy rss-feed-backend --image gcr.io/PROJECT-ID/rss-feed-backend
```

## 📊 Usage Examples

### Fetch RSS Feed (Synchronous)
```bash
curl -X POST http://localhost:8080/fetch-store \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://feeds.bbci.co.uk/news/rss.xml",
    "async": false,
    "force_refresh": false
  }'
```

### Fetch RSS Feed (Asynchronous)
```bash
curl -X POST http://localhost:8080/fetch-store \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://feeds.bbci.co.uk/news/rss.xml",
    "async": true
  }'
```

### Check Job Status
```bash
curl http://localhost:8080/job-status?job_id=your-job-id
```

### Get Feed Items
```bash
curl "http://localhost:8080/items?feed_url=https://feeds.bbci.co.uk/news/rss.xml&limit=10&offset=0"
```

## 🔒 Security Features

### Rate Limiting
- Multi-factor client identification prevents bypass via proxies/VPNs
- Configurable limits per minute and burst capacity
- Automatic cleanup of stale client entries

### URL Validation
- Length limits to prevent DoS attacks (max 2048 characters)
- Private network detection (localhost, private IPs, internal domains)
- Blocks suspicious file extensions (.exe, .php, .js, etc.)
- Script injection detection in query parameters
- RSS feed pattern validation with warnings for non-standard URLs

### CORS Protection
- Environment-specific origin configuration
- Subdomain validation with explicit domain allowlist
- Configurable methods, headers, and credentials
- Proper preflight request handling

## 📈 Monitoring

### Prometheus Metrics
- `rss_feed_fetch_total` - Total feed fetch attempts
- `rss_feed_fetch_duration_seconds` - Feed fetch duration
- `rss_feed_items_count` - Number of items per feed
- `rss_cache_hits_total` - Cache hit statistics
- `rss_async_jobs_total` - Async job statistics

### Distributed Tracing
- OpenTelemetry integration with Jaeger
- Request tracing across service boundaries
- Performance bottleneck identification

### Health Checks
- **Liveness**: Basic service health
- **Readiness**: Dependency health (database, cache)
- **Detailed**: Component-specific health status

## 🧪 Testing

### Run Unit Tests
```bash
go test ./...
```

### Run Integration Tests
```bash
go test -tags=integration ./...
```

### Test Coverage
```bash
go test -cover ./...
```

### Security Tests
```bash
go test -v -run TestEnhancedRateLimiting
go test -v -run TestCORSLogic
```

## 📚 Architecture

### Components
- **Handlers**: HTTP request handlers for different endpoints
- **Services**: Business logic and data processing
- **Cache**: Multi-level caching with adaptive strategies
- **Monitoring**: Metrics, tracing, and alerting
- **Middleware**: Authentication, logging, rate limiting
- **Configuration**: Environment-based configuration management

### Data Flow
1. Client requests hit the middleware layer
2. Rate limiting and CORS validation applied
3. Request routed to appropriate handler
4. Handler processes request using services
5. Results cached and stored in Datastore
6. Response returned with appropriate headers

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🔗 Links

- [API Documentation](http://localhost:8080/swagger/)
- [Prometheus Metrics](http://localhost:8080/metrics)
- [Health Checks](http://localhost:8080/health)
- [GitHub Repository](https://github.com/Nexora-Open-Source/rss-feed-backend)

## 🆘 Support

For support and questions:
- Create an issue in the GitHub repository
- Check the [API documentation](http://localhost:8080/swagger/)
- Review the monitoring dashboard for system status
