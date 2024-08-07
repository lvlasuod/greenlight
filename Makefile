start: 
	go run ./cmd/api -limiter-enabled=true
migrate_up:
	migrate -path ./migrations -database "postgres://greenlight:pa55word@localhost/greenlight?sslmode=disable" up

.PHONY: start migrate_up


