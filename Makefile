start: 
	go run ./cmd/api -limiter-enabled=true

migrate_up:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" up

migrate_down:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" down

pg_login:
	psql.exe "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable"

run_example: # http://localhost:9000/
	go run ./cmd/examples/cors/simple

run_prom: # http://localhost:9090/
	go run ./cmd/examples/prom

	
.PHONY: start migrate_up migrate_down pg_login run_example  


