.PHONY: proto build run clean

proto:
	protoc --go_out=./pkg/pb --go_opt=paths=source_relative \
		--proto_path=./api/proto \
		./api/proto/snakes.proto

build: proto
	go build -o bin/snake ./cmd/snake

run: build
	./bin/snake.exe

clean:
	rm -rf bin/