-- +goose Up
-- +goose StatementBegin
CREATE TABLE link_visits (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    referer TEXT NOT NULL,
    status INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX link_visits_link_id_idx ON link_visits (link_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE link_visits;
-- +goose StatementEnd
