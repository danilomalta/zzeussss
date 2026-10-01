-- Recepcao entre aparelhos da mesma empresa e loja. Sem replicar negocio.
CREATE TABLE sync_incoming_peers (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    receiver_device_id TEXT NOT NULL,
    sender_device_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    sender_public_key BLOB NOT NULL CHECK (length(sender_public_key)=32),
    status TEXT NOT NULL CHECK (status IN ('active','revoked')),
    granted_by TEXT NOT NULL,
    PRIMARY KEY (tenant_id,store_id,receiver_device_id,sender_device_id,event_type),
    CHECK (receiver_device_id <> sender_device_id),
    FOREIGN KEY (tenant_id,store_id,receiver_device_id) REFERENCES devices(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,store_id,sender_device_id) REFERENCES devices(tenant_id,store_id,id),
    FOREIGN KEY (tenant_id,granted_by) REFERENCES memberships(tenant_id,identity_id)
) STRICT;

CREATE TABLE sync_peer_audit (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    receiver_device_id TEXT NOT NULL,
    sender_device_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('grant','revoke')),
    actor_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (tenant_id,store_id,receiver_device_id,sender_device_id,event_type)
        REFERENCES sync_incoming_peers(tenant_id,store_id,receiver_device_id,sender_device_id,event_type),
    FOREIGN KEY (tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;

CREATE TABLE incoming_events (
    tenant_id TEXT NOT NULL,
    store_id TEXT NOT NULL,
    receiver_device_id TEXT NOT NULL,
    sender_device_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    payload_json TEXT NOT NULL CHECK (json_valid(payload_json)),
    message_sha256 TEXT NOT NULL,
    payload_sha256 TEXT NOT NULL,
    sender_signature BLOB NOT NULL CHECK (length(sender_signature)=64),
    receipt_id TEXT NOT NULL UNIQUE,
    receipt_proof BLOB NOT NULL CHECK (length(receipt_proof)=64),
    status TEXT NOT NULL CHECK (status='received'),
    received_at TEXT NOT NULL,
    PRIMARY KEY (tenant_id,store_id,receiver_device_id,sender_device_id,event_id),
    UNIQUE (tenant_id,store_id,receiver_device_id,sender_device_id,operation_id,event_type),
    FOREIGN KEY (tenant_id,store_id,receiver_device_id,sender_device_id,event_type)
        REFERENCES sync_incoming_peers(tenant_id,store_id,receiver_device_id,sender_device_id,event_type)
) STRICT;
