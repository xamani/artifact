# Artifact

REST API for upload/batch-upload/download/list/delete of Linux artifact files. MinIO for storage.

## Architecture

The project keeps HTTP, business rules, and object storage behind small package boundaries:

- `cmd`: process startup, config loading, storage initialization, and graceful shutdown.
- `internal/handler`: Echo routes, API-key middleware, request/response mapping, and streaming downloads.
- `internal/service`: upload/download/list/delete use cases, checksum generation, metadata assembly, batch orchestration.
- `internal/artifact`: domain rules for validation, object key generation, metadata, and rich errors.
- `internal/storage`: object-store port plus the MinIO adapter.

Uploads are spooled to a temporary file while calculating SHA-256, then streamed to MinIO. This avoids keeping artifact bodies fully in memory and still lets the service store checksum metadata before the object is committed.

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

Important config fields:

- `upload.allowed_extensions`: exact allowed artifact extensions. Compound extensions like `.tar.gz` are supported.
- `upload.max_file_size_bytes`: enforced from the multipart metadata and again from the actual bytes read by the server.
- `upload.max_batch_files`: maximum files accepted by `/batch-upload`.
- `naming.prefix`: object key prefix in MinIO.

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

## Object naming

Object keys use:

```text
{prefix}/{yyyy}/{mm}/{os}/{arch}/{sha16}/{original-filename}
```

The SHA-256 prefix reduces collision risk for same-name artifacts while keeping keys readable. Full SHA-256 is also stored in object metadata and returned by upload responses.

## Metadata

Uploaded objects include these user metadata fields:

- `upload-timestamp`
- `os`
- `architecture`
- `hostname`
- `username`
- `sha256`
- `original-filename`
- `mime-type`
- `size`

## Operational notes

- The HTTP server installs recovery, request ID, and structured request logging middleware.
- The process handles `SIGINT` and `SIGTERM` with graceful shutdown.
- The runtime image uses a non-root user.

## Current limitations

- Validation is extension and size based; deep package signature or magic-number verification is not implemented.
- Batch upload runs concurrently, but there is no retry policy around MinIO operations yet.
- API keys are static config values; production deployments should inject secrets through a secret manager or environment-specific configuration.
- Presigned URLs and object versioning are not implemented.
