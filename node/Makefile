.PHONY: clean build plugin
clean:
	rm -rf ./bin

build: clean
	# cd .. && make protoc
	go mod tidy
	go build -o ./bin/node ./cmd

plugin:
	go build -o ./bin/nodeplugin ./plugin/cmd
