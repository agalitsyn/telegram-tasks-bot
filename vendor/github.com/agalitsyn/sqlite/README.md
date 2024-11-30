# sqlite

## Example

```go
db, err := sqlite.Connect("db.sqlite3")
if err != nil {
	log.Fatal(err)
}
defer db.Close()

if err = sqlite.MigrateUp(db, migrations.FS); err != nil {
	log.Printf("ERROR could not apply migrations: %s", err)
	return
}
if cfg.runMigrate {
	os.Exit(0)
}
```
