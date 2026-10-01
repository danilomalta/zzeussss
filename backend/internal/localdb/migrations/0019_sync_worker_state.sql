-- Only operational retry metadata; no private key or business payload.
CREATE TABLE sync_worker_state (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    source_device_id TEXT NOT NULL,
    destination_device_id TEXT NOT NULL,
    event_id TEXT NOT NULL DEFAULT '',
    failures INTEGER NOT NULL DEFAULT 0 CHECK (failures BETWEEN 0 AND 63),
    next_attempt_unix_milli INTEGER NOT NULL DEFAULT 0 CHECK (next_attempt_unix_milli >= 0),
    PRIMARY KEY (tenant_id,store_id,source_device_id),
    FOREIGN KEY (tenant_id,store_id,source_device_id) REFERENCES devices(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,destination_device_id) REFERENCES devices(tenant_id,store_id,id),
    CHECK (source_device_id <> destination_device_id)
) STRICT;
