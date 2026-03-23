package main

// migrateForce is bound by cobra in main.go init().
// It lives here because migrate.go defines the command but the flag
// is registered centrally alongside all other flags.
var migrateForce bool
