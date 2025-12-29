# Examples

This directory contains example projects that can be analyzed by the API Documentation Generator Service.

## Gin Sample

The `gin-sample` directory contains a simple Gin web application with several API endpoints.

### Testing the Service

1. Start the API Doc Generator Service:
   ```bash
   cd ../
   docker-compose up -d
   ```

2. Trigger manual analysis of the sample project:
   ```bash
   curl -X POST http://localhost:8080/api/v1/analyze \
     -H "Content-Type: application/json" \
     -d '{
       "repository_url": "file:///path/to/examples/gin-sample",
       "language": "go-gin"
     }'
   ```

3. The service will:
   - Analyze the code
   - Generate OpenAPI specification
   - Sync to Apifox (if configured)

### Expected Output

The generated OpenAPI spec will include:

- `GET /users` - List all users
- `GET /users/{id}` - Get user by ID
- `POST /users` - Create new user
- `PUT /users/{id}` - Update user
- `DELETE /users/{id}` - Delete user
- `GET /products` - List products
- `GET /products/{id}` - Get product by ID
