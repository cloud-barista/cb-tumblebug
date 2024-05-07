default:
	go build -v -o cb-tumblebug

cc:
	GOOS=linux GOARCH=arm go build -o cb-tumblebug-arm

run:
	./cb-tumblebug

runwithport:
	@echo "Running on port $(PORT) (Usage: make runwithport PORT=8080)"
	./cb-tumblebug --port=$(PORT)

prod:
	@echo "Build for production"
# Note - Using cgo write normal Go code that imports a pseudo-package "C". I may not need on cross-compiling.
# Note - You can find possible platforms by 'go tool dist list' for GOOS and GOARCH
# Note - Using the -ldflags parameter can help set variable values at compile time.
# Note - Using the -s and -w linker flags can strip the debugging information.
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-s -w' -o cb-tumblebug

clean:
	rm -v cb-tumblebug cb-tumblebug-arm

swag swagger:
	~/go/bin/swag i -o ./api/rest/docs
