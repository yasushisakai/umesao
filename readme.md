# required ENV vars

```bash
export AZURE_ENDPOINT=https://app.cognitiveservices.azure.com
export AZURE_KEY=key

export OPENAI_KEY=key

# postgres
export DB_STRING="user=user password='password' host=locahost port=5432 dbname=umesao sslmode=disable"

# minio
export MINIO_USER="minio_user"
export MINIO_PASSWORD="password"
export MINIO_ENDPOINT="localhost:9876"
```

# How to build

[sqlc](https://docs.sqlc.dev/en/latest/index.html) is required 

```bash
# DB transpiling
sqlc compile
sqlc generate

# building
go build -o lookup cmd/lookup/main.go
go build -o upload cmd/upload/main.go
go build -o download cmd/upload/main.go
```
# test

```bash
cd pkg/common
go test
```
