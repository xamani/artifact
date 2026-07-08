# Artifact

REST API for upload/batch-upload/download/list/delete of Linux artifact files. MinIO for storage.

## Run

```bash
docker compose up --build -d
curl http://localhost:8080/health
```

Local without Docker:

```bash
cp configs/config.example.yaml configs/config.yaml
go run ./cmd --config configs/config.yaml
```

## Tests

```bash
go test ./...
go test -tags=integration ./test/integration/...
```

Integration test needs MinIO running (`docker compose up -d`). Unit tests cover validation and config.

## Config

Copy `configs/config.example.yaml` → `configs/config.yaml` for local.

Docker uses `configs/config.docker.yaml`.

## API

All `/api/v1/*` routes need header `X-API-Key` (from `server.api_key` in config).

Optional on upload: `X-Upload-User` — stored in metadata as username.

### Curl examples

Health:

```bash
curl http://localhost:8080/health
```

Upload:

```bash
curl -X POST http://localhost:8080/api/v1/artifacts/upload \
  -H "X-API-Key: api-key-test" \
  -H "X-Upload-User: ci" \
  -F "file=@./pkg.deb"
```

Batch upload:

```bash
curl -X POST http://localhost:8080/api/v1/artifacts/batch-upload \
  -H "X-API-Key: api-key-test" \
  -F "file=@./pkg1.deb" \
  -F "file=@./pkg2.deb"
```

List:

```bash
curl -H "X-API-Key: api-key-test" \
  "http://localhost:8080/api/v1/artifacts/list?prefix=artifacts&limit=20"
```

Download:

```bash
curl -H "X-API-Key: api-key-test" \
  -o out.deb "http://localhost:8080/api/v1/artifacts/artifacts/2026/07/linux/amd64/abc12345/pkg.deb"
```

Metadata:

```bash
curl -H "X-API-Key: api-key-test" \
  "http://localhost:8080/api/v1/artifacts/artifacts/2026/07/linux/amd64/abc12345/pkg.deb/metadata"
```

Delete:

```bash
curl -X DELETE -H "X-API-Key: api-key-test" \
  "http://localhost:8080/api/v1/artifacts/artifacts/2026/07/linux/amd64/abc12345/pkg.deb"
```
