MONGO_CONTAINER=mongopher-mongo

mongo-up:
	docker run -d --name $(MONGO_CONTAINER) -p 27017:27017 mongo:latest --replSet rs0
	sleep 2
	docker exec $(MONGO_CONTAINER) mongosh --eval 'rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]})'

mongo-down:
	docker stop $(MONGO_CONTAINER) && docker rm $(MONGO_CONTAINER)

test:
	go test ./... -v

build:
	go build ./...

todo-server:
	go run ./examples/todo-server/main.go
