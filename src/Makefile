default:
	go build -mod=mod -o cb-tumblebug
cc:
	GOOS=linux GOARCH=arm go build -mod=mod -o cb-tumblebug-arm
run:
	./cb-tumblebug
clean:
	rm -v cb-tumblebug cb-tumblebug-arm
