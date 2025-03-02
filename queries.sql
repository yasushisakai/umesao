-- name: CreateCard :one
INSERT INTO cards DEFAULT
    VALUES
    RETURNING
        id;

-- name: DeleteCard :exec
DELETE FROM cards
WHERE id = $1;

-- name: CreateImage :exec
INSERT INTO images (card_id, filename)
    VALUES ($1, $2);

-- name: CreateMarkdown :exec
INSERT INTO markdown_files (card_id, ver, hash)
    VALUES ($1, $2, $3);

-- name: CreateEmbeddings :exec
INSERT INTO chunks (card_id, ver, idx, model, text, embedding)
    VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLatestMarkdownVersion :one
SELECT
    ver
FROM
    markdown_files
WHERE
    card_id = $1
ORDER BY
    ver DESC
LIMIT 1;

-- name: SearchDistance :many
SELECT
    card_id,
    ver,
    idx,
    model,
    text,
    embedding <-> $1 AS distance
FROM
    chunks
ORDER BY
    distance ASC
LIMIT $2;

-- name: SearchLatestDistance :many
WITH latest_versions AS (
    SELECT
        card_id,
        MAX(ver) AS max_ver
    FROM
        markdown_files
    GROUP BY
        card_id
)
SELECT
    c.card_id,
    c.ver,
    c.idx,
    c.model,
    c.text,
    c.embedding <-> $1 AS distance
FROM
    chunks c
    INNER JOIN latest_versions lv ON c.card_id = lv.card_id
        AND c.ver = lv.max_ver
    ORDER BY
        distance ASC
    LIMIT $2;

-- name: GetCardImage :one
SELECT
    filename
FROM
    images
WHERE
    card_id = $1;

