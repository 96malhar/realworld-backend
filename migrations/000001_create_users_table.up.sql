CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS users
(
    id            bigserial PRIMARY KEY,
    username      text          NOT NULL,
    email         citext UNIQUE NOT NULL,
    password_hash bytea         NOT NULL,
    bio           text,
    image         text,
    version       integer       NOT NULL DEFAULT 1
);
