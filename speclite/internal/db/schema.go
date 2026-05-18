package db

import _ "embed"

//go:embed v1.sql
var schemaV1 string

// schemaVersion is the current expected user_version pragma value.
const schemaVersion = 1
