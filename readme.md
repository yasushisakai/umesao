# How to use

Upload image (a 4'x6' card) with one idea or segment of thought

![](./sample.jpg)

```bash
ume upload image.jpg
```

after you have some amount of cards.

```bash
ume "tokyo"
```

searches all the content that relates to the search query.


```bash
ume edit 2
```

You can edit the content. This will 1. show the image and 2. open neovim to edit.


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
