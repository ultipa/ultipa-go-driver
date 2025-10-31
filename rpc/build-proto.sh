SRC_DIR="./"
DST_DIR="./"

protoc -I=$SRC_DIR \
  --go_out=$DST_DIR \
  --go_opt=Multipa.proto=./ \
  --go-grpc_out=$DST_DIR \
  --go-grpc_opt=Multipa.proto=./ \
  $SRC_DIR/ultipa.proto