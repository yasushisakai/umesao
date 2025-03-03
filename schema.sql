CREATE EXTENSION vector;

CREATE TABLE cards (
    id serial PRIMARY KEY
);

CREATE TABLE images (
    card_id serial REFERENCES cards (id) ON DELETE CASCADE NOT NULL,
    filename text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    method text NOT NULL,
    PRIMARY KEY (card_id, filename)
);

CREATE TABLE markdown_files (
    card_id serial REFERENCES cards (id) ON DELETE CASCADE NOT NULL,
    ver int NOT NULL,
    hash text NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (card_id, ver)
);

-- each markdown_file has multiple embeddings
CREATE TABLE chunks (
    card_id serial REFERENCES cards (id) ON DELETE CASCADE NOT NULL,
    ver int NOT NULL,
    text text NOT NULL,
    idx int NOT NULL, -- 0 is whole text
    -- this might change in the future
    model text NOT NULL,
    -- open ai call can restrict the number of dimensions
    embedding vector (1536),
    PRIMARY KEY (card_id, ver, model, idx),
    FOREIGN KEY (card_id, ver) REFERENCES markdown_files (card_id, ver) ON DELETE CASCADE
);

CREATE INDEX ON chunks USING ivfflat (embedding vector_cosine_ops);

