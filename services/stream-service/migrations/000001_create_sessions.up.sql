-- 000001_create_sessions.up.sql

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE session_status AS ENUM ('playing', 'paused', 'finished');
CREATE TYPE quality_type   AS ENUM ('480p', '720p', '1080p');

CREATE TABLE sessions (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id          UUID NOT NULL,
    movie_id         UUID NOT NULL,
    position_seconds INT DEFAULT 0,
    quality          quality_type DEFAULT '720p',
    status           session_status DEFAULT 'playing',
    started_at       TIMESTAMPTZ DEFAULT NOW(),
    updated_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE subtitles (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    movie_id   UUID NOT NULL,
    lang       VARCHAR(10) NOT NULL,   -- 'en', 'ru', 'kz'
    label      VARCHAR(50) NOT NULL,   -- 'English', 'Русский'
    file_url   VARCHAR(500) NOT NULL,
    UNIQUE(movie_id, lang)
);

CREATE INDEX idx_sessions_user_id  ON sessions(user_id);
CREATE INDEX idx_sessions_movie_id ON sessions(movie_id);
CREATE INDEX idx_sessions_status   ON sessions(status);
CREATE INDEX idx_subtitles_movie   ON subtitles(movie_id);
