
build:
	go build -o release/linux/SSMAgent .

run: build
	su ssm -c 'release/linux/SSMAgent --name "Test" --apikey "AGT-API-QXPMA6MTKXTFNVZWF07SMLCQLI5GSHUA" --url "https://api-ssmcloud-dev.hostxtra.co.uk" --grpcaddr "grpc-ssmcloud-dev.hostxtra.co.uk"'