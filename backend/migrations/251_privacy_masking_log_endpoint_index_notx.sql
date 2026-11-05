CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_privacy_masking_log_endpoint_detected_at ON privacy_masking_log (endpoint, detected_at DESC);
