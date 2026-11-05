PG_DSN    ?= postgres://localhost:5432/university_lab2
MONGO_URI ?= mongodb://localhost:27017
STUDENTS  ?= 50000

export PG_DSN MONGO_URI

.PHONY: services load bench redis redis-demo mongo-queries sql-queries all

services:
	brew services start mongodb/brew/mongodb-community
	brew services start redis

load:
	cd app && go run ./cmd/load -students $(STUDENTS) -sql ../sql -out ../results/load.json

bench:
	cd app && go run ./cmd/bench -out ../results

mongo-queries:
	mongosh --quiet university_lab2 mongo/queries.js

sql-queries:
	psql -X -d $(PG_DSN) -f sql/queries.sql

redis:
	bash redis/basic_queries.sh

redis-demo:
	cd app && go run ./cmd/redis

all: load bench redis redis-demo
