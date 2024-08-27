start: 
	go run ./cmd/api -limiter-enabled=true

migrate_up:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" up

migrate_down:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" down

pg_login:
	psql.exe "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable"
	
.PHONY: start migrate_up migrate_down pg_login 


