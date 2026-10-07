package migrations

import _ "embed"

// OnlineSessionsSQL is the only legacy PostgreSQL migration this delivery runs.
//
//go:embed 000005_online_sessions.sql
var OnlineSessionsSQL string

//go:embed 000006_online_password_changes.sql
var OnlinePasswordChangesSQL string

//go:embed 000007_online_recovery_keys.sql
var OnlineRecoveryKeysSQL string
