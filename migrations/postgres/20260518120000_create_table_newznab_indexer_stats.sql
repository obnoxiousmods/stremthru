-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS "public"."newznab_indexer_stats" (
    "id" text PRIMARY KEY,
    "indexer_id" bigint NOT NULL DEFAULT 0,
    "operation" text NOT NULL DEFAULT '',
    "error_type" text NOT NULL DEFAULT '',
    "error" text,
    "http_status" int NOT NULL DEFAULT 0,
    "latency_ms" double precision NOT NULL DEFAULT 0,
    "result_count" bigint NOT NULL DEFAULT 0,
    "bytes" bigint NOT NULL DEFAULT 0,
    "cat" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX newznab_indexer_stats_idx_indexer_cat ON "public"."newznab_indexer_stats" ("indexer_id", "cat");
CREATE INDEX newznab_indexer_stats_idx_cat ON "public"."newznab_indexer_stats" ("cat");

ALTER TABLE "public"."nzb_info" ADD COLUMN "indexer_id" bigint REFERENCES "public"."newznab_indexer"("id") ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE "public"."nzb_info" DROP COLUMN IF EXISTS "indexer_id";

DROP TABLE IF EXISTS "public"."newznab_indexer_stats";
-- +goose StatementEnd
