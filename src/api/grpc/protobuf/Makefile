# https://github.com/gogo/protobuf/blob/master/test/example/Makefile
#
regenerate:
#	(protoc -I=. -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf -I=$(GOPATH)/src --gogo_out=. cbtumblebug.proto)
	protoc -I . -I $(GOPATH)/src/github.com/gogo/protobuf/protobuf -I $(GOPATH)/src --gofast_out=plugins=grpc,paths=source_relative,Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types:. cbtumblebug.proto
