go run cmd/cored/main.go start \
  --home /home/app \
  --rpc.laddr tcp://0.0.0.0:26657 \
  --p2p.laddr tcp://0.0.0.0:26656 \
  --grpc.address 0.0.0.0:9090 \
  --grpc-web.address 0.0.0.0:9091 \
  --rpc.pprof_laddr 0.0.0.0:6060
