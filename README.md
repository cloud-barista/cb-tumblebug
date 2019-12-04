# CB-Tumblebug (Multi-Cloud Infra Service Management)
Proof of Concepts for the Cloud-Barista Multi-Cloud Project.

***

## [목차]

1. [설치 개요](#설치-개요)
2. [설치 절차](#설치-절차)
3. [설치 & 실행 상세 정보](#설치--실행-상세-정보)

***

## [설치 개요]
- 설치 환경: 리눅스(검증시험:Ubuntu 18.04)

## [설치 절차]

- Go 설치 & Git 설치
- 환경 변수 설정
- MCISM 소스 다운로드 (Git clone MCISM)
- 의존 라이브러리 다운로드
  - Cloud-Barista alliance 설치 (CB-Store, CB-Log, CB-Spider)
  - 기타 라이브러리
- mcism 빌드 (make)
- mcism 실행 (make run)

## [설치 & 실행 상세 정보]

- Git 설치
  - `# apt update`
  - `# apt install git`

- Go 설치
  - https://golang.org/doc/install (2019년 11월 현재 `apt install golang` 으로 설치하면 1.10 설치됨. 이 링크에서 1.12 이상 버전으로 설치할 것)
  - `wget https://dl.google.com/go/go1.13.4.linux-amd64.tar.gz`
  - `tar -C /usr/local -xzf go1.13.4.linux-amd64.tar.gz`
  - `.bashrc` 파일 하단에 다음을 추가: `export PATH=$PATH:/usr/local/go/bin`


- `.bashrc` 에 기재한 내용을 적용하기 위해, 다음 중 하나를 수행
  - bash 재기동
  - `source ~/.bashrc`
  - `. ~/.bashrc`

- MCISM 소스 다운로드
  - `# go get -u -v github.com/cloud-barista/cb-tumblebug`

- 의존 라이브러리 다운로드
  - etcd 설치 및 실행
  ```Shell
  # apt install etcd-server
  # etcd --version
  # ETCD_IP=<ETCD-Host-IPAddress>
  # etcd --name etcd-01 --initial-advertise-peer-urls http://$ETCD_IP:2380 --listen-peer-urls http://$ETCD_IP:2380 --listen-client-urls http://$ETCD_IP:2379,http://127.0.0.1:2379 --advertise-client-urls http://$ETCD_IP:2379 --initial-cluster-token "etcd-cluster-1" --initial-cluster etcd-01=http://$ETCD_IP:2380 --initial-cluster-state new  &
  ```

  - Cloud-Barista alliance 설치 (CB-Log)
    - `go get -u -v github.com/cloud-barista/cb-log`
    - https://github.com/cloud-barista/cb-log README를 참고하여 설치 및 설정
  
  - Cloud-Barista alliance 설치 (CB-Store)
    - `go get -u -v github.com/cloud-barista/cb-store`
    - https://github.com/cloud-barista/cb-store README를 참고하여 설치 및 설정

  - Cloud-Barista alliance 설치 (CB-Spider)
    - `go get -u -v github.com/cloud-barista/cb-spider`
    - https://github.com/cloud-barista/cb-spider README를 참고하여 설치 및 설정

- CB-Tumblebug 실행에 필요한 환경변수 설정
  - `source setup.env` (cb-tumblebug/conf 에 setup.env)
  - cb-tumblebug/conf 에 store_conf.yaml 내용 확인 및 설정 (CB-Store 설정)
    - storetype 지정 (NUTSDB 또는 ETCD 지정)
    - NUTSDB(local DB) 설정시 DB 데이터가 포함된 주소 지정이 필요 (기본은 cb-tumblebug/meta_db/dat 에 파일로 추가됨)
  - cb-tumblebug/conf 에 log_conf.yaml 내용 확인 및 설정 (CB-Log 설정)


### mcism 빌드

```Shell
# cd ~/go/src/github.com/cloud-barista/cb-tumblebug/src
# ./make
```
- 패키지 관련 오류 발생 시, `go get` 명령을 통해 부족한 패키지를 추가

### mcism 실행
- 만약 AWS 관련 작업 시 에러가 발생하면 다음을 실행
```Shell
# sudo apt-get --yes install ntpdate
# sudo ntpdate 0.amazon.pool.ntp.org
```

- 만약 `“panic: /debug/requests is already registered. You may have two independent copies of golang.org/x/net/trace in your binary, trying to maintain separate state. This may involve a vendored copy of golang.org/x/net/trace.”` 에러가 발생하면 다음을 실행 (mcism rebuild 가 필요할 수도 있음)
```Shell
# rm -rf $GOPATH/src/go.etcd.io/etcd/vendor/golang.org/x/net/trace
```

- `# ./make run` (또는 `# go run mcism.go`)
  - CB-Tumblebug API server가 실행됨

### mcism 테스트 방법

- mcism 의 REST API를 사용하여 테스트
  - Cloud-Barista 사용자 API 리스트: https://github.com/cloud-barista/docs/blob/master/API-Specifications/User-REST-API(v0.30).md
  - 멀티 클라우드 네임스페이스 관리 API를 통해서 Namespace 생성
  - 멀티 클라우드 인프라 자원(MCIR) 관리 API를 통해서 MCIS 및 VM 생성을 위한 자원 생성
  - 멀티 클라우드 인프라 서비스(MCIS) 관리 API를 통해서 MCIS 생성, 조회, 제어, 종료
