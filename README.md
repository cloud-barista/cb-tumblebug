# cb-tumblebug (Multi-Cloud Infra Service Management)
Proof of Concepts for the Cloud-Barista Multi-Cloud Project.

***

## [목차]

1. [설치 개요](#설치-개요)
2. [설치 절차](#설치-절차)
3. [설치 & 실행 상세 정보](#설치--실행-상세-정보)

***

## [설치 개요]
- 설치 환경: Ubuntu 18.04

## [설치 절차]

- Go 설치 & Git 설치
- 환경 변수 설정
- MCISM 소스 다운로드 (Git clone MCISM)
- 의존 라이브러리 다운로드
  - Cloud-Barista alliance 설치(CB-Store)
  - 클라우드 Go 클라이언트 라이브러리
  - 기타 라이브러리
- mcism 빌드
  - mcism_agent 빌드
  - mcism_server 빌드
- mcism_server 실행

## [설치 & 실행 상세 정보]
- Go 설치 & Git 설치
  - https://golang.org/doc/install
  - `# apt install git`
- 환경 변수 설정
  - `~/.bashrc` (또는 `~/.zshrc`) 하단에 아래 내용 추가
  ```Shell
  export PATH=$PATH:/usr/local/go/bin
  export GOPATH=$HOME/go
  export PATH=$PATH:$HOME/go/src/github.com/protobuf/bin
  export AZURE_AUTH_LOCATION=~/.azure/azure.auth
  ```
> 1행: Go 를 Ubuntu 패키지로 설치한다면 필요 없을 것임  
2행: Go 를 Ubuntu 패키지로 설치한다면 필요 없을 것임  
3행: golang-goprotobuf-dev 를 Ubuntu 패키지로 설치한다면 필요 없을 것임

- `.bashrc` 에 기재한 내용을 적용하기 위해, 다음 중 하나를 수행
  - logoff 후 다시 login
  - `source ~/.bashrc`
  - `. ~/.bashrc`

- CB-Tumblebug 실행에 필요한 환경변수 설정
  - `source setup.env`

- MCISM 소스 다운로드
  - `# go get github.com/cloud-barista/cb-tumblebug`
  > 다음과 같은 메시지가 나오기는 함:  
  `“package github.com/cloud-barista/cb-tumblebug: no Go files in /root/go/src/github.com/cloud-barista/cb-tumblebug”`

  > `# go get github.com/cloud-barista/cb-tumblebug` 명령을 실행하면, 다음의 명령들을 실행한 것과 같은 효과를 냄  
  > `# mkdir ~/go/src/github.com/cloud-barista`  
  > `# cd ~/go/src/github.com/cloud-barista`  
  > `# git clone https://github.com/cloud-barista/cb-tumblebug.git`

- 의존 라이브러리 다운로드
  - etcd 설치 및 실행
  ```Shell
  # apt install etcd-server
  # etcd --version
  # ETCD_IP=<ETCD-Host-IPAddress>
  # etcd --name etcd-01 --initial-advertise-peer-urls http://$ETCD_IP:2380 --listen-peer-urls http://$ETCD_IP:2380 --listen-client-urls http://$ETCD_IP:2379,http://127.0.0.1:2379 --advertise-client-urls http://$ETCD_IP:2379 --initial-cluster-token "etcd-cluster-1" --initial-cluster etcd-01=http://$ETCD_IP:2380 --initial-cluster-state new  &
  ```
  - Cloud-Barista alliance 설치(cb-store)
  https://github.com/cloud-barista/cb-store
  README를 참고하여 설치 및 설정
  - 클라우드 Go 클라이언트 관련 라이브러리
  ```Shell
  # go get -u -v github.com/aws/aws-sdk-go
  # go get -u -v cloud.google.com/go
  # go get -u -v github.com/Azure/azure-sdk-for-go
  # go get -u -v github.com/Azure/go-autorest/autorest
  ```
  - 기타 라이브러리 다운로드
  ```Shell
  # go get -u -v github.com/revel/revel
  # go get -u -v go.etcd.io/etcd/clientv3
  # go get -u -v github.com/bramvdbogaerde/go-scp
  # go get -u -v github.com/dimchansky/utfbom github.com/mitchellh/go-homedir
  # go get -u -v golang.org/x/oauth2 gopkg.in/yaml.v3
  # go get -u -v github.com/labstack/echo
  # go get -u -v github.com/google/uuid
  ```

### mcism 빌드
- mcism_agent 빌드
```Shell
# apt install golang-goprotobuf-dev
# bash ~/go/src/github.com/cloud-barista/cb-tumblebug/1.agent_protoc_build.sh
```
- mcism_server 빌드
```Shell
# cd ~/go/src/github.com/cloud-barista/cb-tumblebug/mcism_server
# go build
```
> 패키지 관련 오류 발생 시, `go get` 명령을 통해 부족한 패키지를 추가

### mcism_server 실행
- 만약 AWS 관련 작업 시 에러가 발생하면 다음을 실행
```Shell
# sudo apt-get --yes install ntpdate
# sudo ntpdate 0.amazon.pool.ntp.org
```

- 만약 `“panic: /debug/requests is already registered. You may have two independent copies of golang.org/x/net/trace in your binary, trying to maintain separate state. This may involve a vendored copy of golang.org/x/net/trace.”` 에러가 발생하면 다음을 실행 (mcism_server rebuild 가 필요할 수도 있음)
```Shell
# rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
```

- `# ./mcism_server` (또는 `# go run mcism_server.go`)
  - API server가 실행됨

