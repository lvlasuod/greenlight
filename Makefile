start: 
	go run ./cmd/api -limiter-enabled=true
migrate_up:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" up
pg_login:
	psql.exe "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable"
	
.PHONY: start migrate_up pg_login


