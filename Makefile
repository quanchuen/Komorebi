MIGRATE_URL ?= postgres://osm_dev:osm_dev@localhost:5432/cyclist_map_dev?sslmode=disable

.PHONY: migrate-up migrate-down migrate-create

migrate-up:
	migrate -path migrations -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

migrate-create:
	@read -p "Name: " name; \
	migrate create -ext sql -dir migrations -seq -digits 6 $$name
