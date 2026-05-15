-- 000001_create_content.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE genres (
    id   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) UNIQUE NOT NULL
);

CREATE TABLE movies (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title        VARCHAR(255) NOT NULL,
    description  TEXT,
    year         INT,
    genre_id     UUID REFERENCES genres(id),
    video_url    VARCHAR(500),
    poster_url   VARCHAR(500),
    duration_sec INT DEFAULT 0,
    views        INT DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE ratings (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    movie_id   UUID NOT NULL REFERENCES movies(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL,
    score      FLOAT NOT NULL CHECK (score >= 1 AND score <= 10),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(movie_id, user_id)
);

-- Seed genres
INSERT INTO genres (name) VALUES
    ('Action'), ('Comedy'), ('Drama'),
    ('Horror'), ('Sci-Fi'), ('Documentary');

CREATE INDEX idx_movies_genre_id ON movies(genre_id);
CREATE INDEX idx_movies_title    ON movies USING gin(to_tsvector('english', title));
CREATE INDEX idx_ratings_movie_id ON ratings(movie_id);
