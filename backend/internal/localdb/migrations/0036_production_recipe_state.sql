CREATE TABLE production_recipe_states (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, recipe_id TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('active','inactive')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 PRIMARY KEY(tenant_id,store_id,recipe_id),
 FOREIGN KEY(tenant_id,store_id,recipe_id) REFERENCES production_recipes(tenant_id,store_id,id)
) STRICT;
CREATE TABLE production_recipe_state_events (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, recipe_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)),
 before_status TEXT NOT NULL CHECK(before_status IN ('active','inactive')),
 after_status TEXT NOT NULL CHECK(after_status IN ('active','inactive')),
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 reason TEXT NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 255), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,recipe_id) REFERENCES production_recipes(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
