-- Roles y permisos por tenant; asignacion de roles a usuarios; relacion rol<->permiso.

BEGIN;

CREATE TABLE IF NOT EXISTS rbac_role (
  tenant_id UUID NOT NULL,
  role TEXT NOT NULL,
  description TEXT,
  PRIMARY KEY (tenant_id, role)
);

CREATE TABLE IF NOT EXISTS rbac_perm (
  tenant_id UUID NOT NULL,
  perm TEXT NOT NULL,
  description TEXT,
  PRIMARY KEY (tenant_id, perm)
);

CREATE TABLE IF NOT EXISTS rbac_role_perm (
  tenant_id UUID NOT NULL,
  role TEXT NOT NULL,
  perm TEXT NOT NULL,
  PRIMARY KEY (tenant_id, role, perm),
  FOREIGN KEY (tenant_id, role) REFERENCES rbac_role(tenant_id, role) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, perm) REFERENCES rbac_perm(tenant_id, perm) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS rbac_user_role (
  tenant_id UUID NOT NULL,
  user_id  UUID NOT NULL,
  role     TEXT NOT NULL,
  PRIMARY KEY (tenant_id, user_id, role),
  FOREIGN KEY (tenant_id, role) REFERENCES rbac_role(tenant_id, role) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES app_user(id) ON DELETE CASCADE
);
-- Indices de ayuda
CREATE INDEX IF NOT EXISTS idx_rbac_user_role_user ON rbac_user_role(user_id);
CREATE INDEX IF NOT EXISTS idx_rbac_role_perm_perm ON rbac_role_perm(tenant_id, perm);

COMMIT;
