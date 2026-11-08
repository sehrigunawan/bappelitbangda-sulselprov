-- Privilege tests. Tables the app must not edit or delete (for example the access log, NFR-08) get a check here.
-- Skeleton baseline: the runtime role must not be able to create objects.
DO $$
BEGIN
  ASSERT NOT has_schema_privilege('app', 'public', 'CREATE'),
    'role app must not be able to create objects in schema public';
  ASSERT NOT has_database_privilege('app', current_database(), 'CREATE'),
    'role app must not be able to create schemas';
END
$$;

-- D4: role app must not DELETE users or data_keys, and must not UPDATE data_keys.id or data_keys.created_at.
-- Positive checks: the grants the BE lane needs (recovery code use, KEK rewrap, session deletion) are present.
DO $$
BEGIN
  ASSERT NOT has_table_privilege('app', 'users', 'DELETE'),
    'D4: role app must not be able to DELETE from users (deactivate via disabled_at)';
  ASSERT NOT has_table_privilege('app', 'data_keys', 'DELETE'),
    'D4: role app must not be able to DELETE from data_keys';
  ASSERT NOT has_column_privilege('app', 'data_keys', 'id', 'UPDATE'),
    'D4: role app must not be able to UPDATE data_keys.id';
  ASSERT NOT has_column_privilege('app', 'data_keys', 'created_at', 'UPDATE'),
    'D4: role app must not be able to UPDATE data_keys.created_at';
  ASSERT has_column_privilege('app', 'recovery_codes', 'used_at', 'UPDATE'),
    'D4: role app must be able to UPDATE recovery_codes.used_at';
  ASSERT has_column_privilege('app', 'data_keys', 'kek_id', 'UPDATE'),
    'D4: role app must be able to UPDATE data_keys.kek_id for KEK rewrap';
  ASSERT has_table_privilege('app', 'sessions', 'DELETE'),
    'D4: role app must be able to DELETE from sessions (logout and expiry)';
END
$$;
