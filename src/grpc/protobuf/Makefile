# https://github.com/gogo/protobuf/blob/master/test/example/Makefile
#
regenerate:
	(protoc -I=. -I=$(GOPATH)/src/github.com/gogo/protobuf/protobuf -I=$(GOPATH)/src --gogo_out=. cbtumblebug.proto)
