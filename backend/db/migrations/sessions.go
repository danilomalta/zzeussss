package migrations

import _ "embed"

// OnlineSessionsSQL is the only legacy PostgreSQL migration this delivery runs.
//
//go:embed 000005_online_sessions.sql
var OnlineSessionsSQL string
