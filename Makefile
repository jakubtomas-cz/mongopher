MONGO_CONTAINER=mongopher-mongo

mongo-up:
	docker run -d --name $(MONGO_CONTAINER) -p 27017:27017 mongo:latest

mongo-down:
	docker stop $(MONGO_CONTAINER) && docker rm $(MONGO_CONTAINER)

test:
	go test ./... -v

build:
	go build ./...
