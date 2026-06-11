-- Aggregated click counts per short code. One row per code; the worker
-- increments click_count via an idempotent-per-row upsert.
CREATE TABLE IF NOT EXISTS click_stats (
    code            VARCHAR(16) NOT NULL,
    click_count     BIGINT      NOT NULL DEFAULT 0,
    last_clicked_at DATETIME    NULL,
    updated_at      TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;