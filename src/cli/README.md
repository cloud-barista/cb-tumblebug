# CB-Tumblebug CLI (가칭:tbctl)
CB-Tumblebug의 기본적인 시나리오 테스트를 지원하기 위한 CLI 도구

***

## [목차]

1. [CLI 개요](#CLI-개요)
2. [CLI 사용 방법](#CLI-사용-방법)
3. [CLI 개발 방법](#CLI-개발-방법)

***

## [CLI 개요]
- CB-Tumblebug의 기본적인 시나리오 테스트를 지원하기 위한 CLI (목적 변경 가능)
- CLI 기본 코드는 cobra 프로젝트 (https://github.com/spf13/cobra) 를 통해 자동 생성함
- 현황: 개발 진행중

## [CLI 사용 방법]

- cb-tumblebug 기본 환경 변수 설정
  - cb-tumblebug/conf 에서 source setup.env
- cb-tumblebug/src/cli 에서 `go run main.go [명령어] [옵션]`
  - 예제) `go run main.go --help`
  - 예제) `go run main.go create -f mcis.json -t mcis`
  - 예제) `go run main.go get -f ns.json -t ns`

<!-- son@son:~/go/src/github.com/cloud-barista/cb-tumblebug/src/cli$ go run main.go --help
[DB file path] /home/son/go/src/github.com/cloud-barista/cb-tumblebug/meta_db/dat
Setting config file: A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.

Usage:
  cli [flags]
  cli [command]

Available Commands:
  create      create based on file
  delete      A brief description of your command
  get         A brief description of your command
  help        Help about any command
  list        A brief description of your command

Flags:
      --config string   config file (default is $HOME/.cli.yaml)
  -h, --help            help for cli
  -t, --toggle          Help message for toggle

Use "cli [command] --help" for more information about a command. -->


## [CLI 개발 방법]

- Cobra 설치 및 사용방법 확인
  - `go get -u github.com/spf13/cobra/cobra` cobra 설치
  - `$GOPATH/bin/cobra init ./cb-tumblebug/src/cli --pkg-name cli` 새로운 cli 프로젝트 생성시 명령어 (cobra가 코드를 자동 생성)
  - `$GOPATH/bin/cobra add create` 프로젝트 내에 신규 cli 명령어(ex:create) 추가 (cobra가 create 관련 코드를 자동 생성)
  - `cli/cmd/create.go` 를 수정 개발하면 create 명령어 실행 내용을 변경할 수 있음
  - [cobra_사용_방법](https://towardsdatascience.com/how-to-create-a-cli-in-golang-with-cobra-d729641c7177) 참고
