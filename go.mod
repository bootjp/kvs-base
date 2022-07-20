module github.com/bootjp/kvs-base

replace github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.3
replace github.com/pingcap/tidb => github.com/pingcap-incubator/tinysql v0.0.0-20200518090433-a7d00f9e6aa7
replace google.golang.org/grpc => google.golang.org/grpc v1.25.1

go 1.18

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Connor1996/badger v1.5.1-0.20200306031920-9bbcbd8ba570
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/docker/go-units v0.4.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.3.4
	github.com/google/btree v1.0.0
	github.com/juju/errors v0.0.0-20220203013757-bd733f3c86b9
	github.com/opentracing/opentracing-go v1.0.2
	github.com/petar/GoLLRB v0.0.0-20190514000832-33fb24c13b99
	github.com/pingcap/check v0.0.0-20200212061837-5e12011dc712
	github.com/pingcap/errcode v0.0.0-20180921232412-a1a7271709d9
	github.com/pingcap/errors v0.11.5-0.20190809092503-95897b64e011
	github.com/pingcap/log v0.0.0-20200117041106-d28c14d3b1cd
	github.com/pingcap/tidb v2.0.11+incompatible
	github.com/pingcap/tipb v0.0.0-20200212061130-c4d518eb1d60
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/sirupsen/logrus v1.2.0
	github.com/stretchr/testify v1.4.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20191023171146-3cf2f69b5738
	go.uber.org/zap v1.14.0
	golang.org/x/net v0.0.0-20220708220712-1185a9018129
	google.golang.org/grpc v1.48.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
)

require (
	github.com/DataDog/zstd v1.4.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/coocood/bbloom v0.0.0-20190830030839-58deb6228d64 // indirect
	github.com/coocood/rtutil v0.0.0-20190304133409-c84515f646f2 // indirect
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20180511133405-39ca1b05acc7 // indirect
	github.com/cznic/mathutil v0.0.0-20181122101859-297441e03548 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/dgryski/go-farm v0.0.0-20190423205320-6a90982ecee2 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/flatbuffers v1.12.1 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/gorilla/websocket v0.0.0-20170926233335-4201258b820c // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190118093823-f849b5445de4 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.12.1 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/json-iterator/go v1.1.7 // indirect
	github.com/juju/testing v1.0.1 // indirect
	github.com/klauspost/compress v1.12.3 // indirect
	github.com/klauspost/cpuid v1.2.1 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.1 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/ncw/directio v1.0.4 // indirect
	github.com/ngaut/log v0.0.0-20180314031856-b8e36e7ba5ac // indirect
	github.com/pingcap/failpoint v0.0.0-20200210140405-f8f9fb234798 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.0.0 // indirect
	github.com/prometheus/client_model v0.0.0-20190812154241-14fe0d1b01d4 // indirect
	github.com/prometheus/common v0.4.1 // indirect
	github.com/prometheus/procfs v0.0.2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20200410134404-eec4a21b6bb0 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20170815181823-89b8d40f7ca8 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/atomic v1.6.0 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3 // indirect
	golang.org/x/lint v0.0.0-20190930215403-16217165b5de // indirect
	golang.org/x/mod v0.3.0 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20181108054448-85acf8d2951c // indirect
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20190927181202-20e1ac93f88c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	honnef.co/go/tools v0.0.1-2019.2.3 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)
