-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS `newznab_indexer_stats` (
    `id` varchar PRIMARY KEY,
    `indexer_id` int NOT NULL DEFAULT 0,
    `operation` varchar NOT NULL DEFAULT '',
    `error_type` varchar NOT NULL DEFAULT '',
    `error` text,
    `http_status` int NOT NULL DEFAULT 0,
    `latency_ms` real NOT NULL DEFAULT 0,
    `result_count` int NOT NULL DEFAULT 0,
    `bytes` int NOT NULL DEFAULT 0,
    `cat` datetime NOT NULL DEFAULT (unixepoch())
);

CREATE INDEX newznab_indexer_stats_idx_indexer_cat ON `newznab_indexer_stats` (`indexer_id`, `cat`);
CREATE INDEX newznab_indexer_stats_idx_cat ON `newznab_indexer_stats` (`cat`);

ALTER TABLE `nzb_info` ADD COLUMN `indexer_id` int REFERENCES `newznab_indexer`(`id`) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE `nzb_info` DROP COLUMN `indexer_id`;

DROP TABLE IF EXISTS `newznab_indexer_stats`;
-- +goose StatementEnd
