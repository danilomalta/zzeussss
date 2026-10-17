-- Published versions are append-only through the production service.
CREATE TABLE production_recipes (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 PRIMARY KEY(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id) REFERENCES stores(tenant_id,id)
) STRICT;
CREATE TABLE production_recipe_versions (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, recipe_id TEXT NOT NULL,
 revision INTEGER NOT NULL CHECK(revision BETWEEN 1 AND 2147483647),
 version_id TEXT NOT NULL, name TEXT NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 255),
 output_product_id TEXT NOT NULL, output_unit TEXT NOT NULL,
 yield_milli INTEGER NOT NULL CHECK(yield_milli BETWEEN 1 AND 9007199254740991),
 device_id TEXT NOT NULL, operation_id TEXT NOT NULL, actor_id TEXT NOT NULL,
 request_json TEXT NOT NULL CHECK(json_valid(request_json)), created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,store_id,recipe_id,revision),
 UNIQUE(tenant_id,store_id,version_id),
 UNIQUE(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,recipe_id) REFERENCES production_recipes(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,output_product_id) REFERENCES products(tenant_id,id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE TABLE production_recipe_ingredients (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, version_id TEXT NOT NULL,
 product_id TEXT NOT NULL, unit TEXT NOT NULL,
 quantity_milli INTEGER NOT NULL CHECK(quantity_milli BETWEEN 1 AND 9007199254740991),
 PRIMARY KEY(tenant_id,store_id,version_id,product_id),
 FOREIGN KEY(tenant_id,store_id,version_id) REFERENCES production_recipe_versions(tenant_id,store_id,version_id),
 FOREIGN KEY(tenant_id,product_id) REFERENCES products(tenant_id,id)
) STRICT;
CREATE TABLE production_recipe_audit (
 tenant_id TEXT NOT NULL, store_id TEXT NOT NULL, device_id TEXT NOT NULL,
 operation_id TEXT NOT NULL, actor_id TEXT NOT NULL, recipe_id TEXT NOT NULL,
 version_id TEXT NOT NULL, revision INTEGER NOT NULL, created_at TEXT NOT NULL,
 PRIMARY KEY(tenant_id,device_id,operation_id),
 FOREIGN KEY(tenant_id,store_id,version_id) REFERENCES production_recipe_versions(tenant_id,store_id,version_id),
 FOREIGN KEY(tenant_id,store_id,device_id) REFERENCES devices(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,actor_id) REFERENCES memberships(tenant_id,identity_id)
) STRICT;
CREATE INDEX production_recipe_versions_page ON production_recipe_versions(tenant_id,store_id,recipe_id,revision);
