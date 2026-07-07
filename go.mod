module github.com/SatisfactoryServerManager/SSMAgent

go 1.26

require (
	github.com/SatisfactoryServerManager/ssmcloud-resources v0.0.52
	github.com/hpcloud/tail v1.0.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	go.mongodb.org/mongo-driver v1.17.6
	golang.org/x/mod v0.35.0
	google.golang.org/grpc v1.81.1
	gopkg.in/ini.v1 v1.67.0
)

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/stretchr/testify v1.10.0 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.mongodb.org/mongo-driver/v2 v2.6.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
)

replace github.com/SatisfactoryServerManager/ssmcloud-resources => ../ssmcloud-resources
