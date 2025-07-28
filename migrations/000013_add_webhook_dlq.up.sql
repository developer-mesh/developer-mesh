-- Create webhook dead letter queue table
CREATE TABLE IF NOT EXISTS webhook_dlq (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,
    error_message TEXT NOT NULL,
    retry_count INTEGER DEFAULT 0,
    last_retry_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    metadata JSONB
);

-- Create indexes for common queries
CREATE INDEX idx_webhook_dlq_event_id ON webhook_dlq (event_id);
CREATE INDEX idx_webhook_dlq_status ON webhook_dlq (status);
CREATE INDEX idx_webhook_dlq_created_at ON webhook_dlq (created_at);
CREATE INDEX idx_webhook_dlq_status_created ON webhook_dlq (status, created_at);

-- Add comment
COMMENT ON TABLE webhook_dlq IS 'Dead letter queue for failed webhook events';
COMMENT ON COLUMN webhook_dlq.status IS 'Status: pending, retrying, failed, resolved';