default:
	go build -mod=mod -o cb-tumblebug
cc:
	GOOS=linux GOARCH=arm go build -mod=mod -o cb-tumblebug-arm
swag swagger:
	~/go/bin/swag i
proto protobuf pb:
	cd api/grpc/protobuf && $(MAKE) regenerate
run:
	./cb-tumblebug
clean:
	rm -v cb-tumblebug cb-tumblebug-arm
