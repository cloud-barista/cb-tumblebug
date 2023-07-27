default:
	go build -o cb-tumblebug
cc:
	GOOS=linux GOARCH=arm go build -o cb-tumblebug-arm
swag swagger:
	~/go/bin/swag i -o ./api/rest/docs
run:
	./cb-tumblebug
runwithport:
	./cb-tumblebug --port=$(PORT)
clean:
	rm -v cb-tumblebug cb-tumblebug-arm
