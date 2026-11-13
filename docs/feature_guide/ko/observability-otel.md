# 관측성 (Observability, OpenTelemetry)

CB-Tumblebug 노드의 메트릭·로그 파이프라인 가이드입니다. CB-Tumblebug이 프로비저닝하거나 등록한 OTLP 엔드포인트(와 Grafana), 그리고 노드가 [init script](init-script.md) 템플릿 `initscript-otel-agent`로 설치하는 OpenTelemetry Collector 에이전트를 다룹니다.

## 📑 목차

1. [개요](#개요)
2. [아키텍처](#아키텍처)
3. [스택 구성](#스택-구성)
4. [노드에서 데이터 보내기](#노드에서-데이터-보내기)
5. [대시보드와 뷰어 계정](#대시보드와-뷰어-계정)
6. [GPU 메트릭](#gpu-메트릭)
7. [API 레퍼런스](#api-레퍼런스)
8. [규모·보안·제약](#규모보안제약)

---

## 개요

CB-Tumblebug은 관측성 스택 등록 정보를 하나 보관합니다. 노드가 내보낼 OTLP/HTTP 엔드포인트, bearer 토큰, CA(자체서명 스택), Grafana URL입니다. 모든 init script에 `TB_OTEL_ENDPOINT`, `TB_OTEL_TOKEN`, `TB_OTEL_CA_B64`로 전달되지만, Infra 요청에 `initscript-otel-agent` 템플릿을 넣지 않으면 아무것도 설치되지 않습니다. 텔레메트리에는 노드의 CB-Tumblebug 식별자(`tb.ns`, `tb.infra`, `tb.nodegroup`, `tb.node`, `cloud.provider`, `cloud.region`)가 붙어 대시보드가 Infra·NodeGroup·노드 단위로 바로 묶입니다.

| 기능 | 설명 |
|------|------|
| 관리형 스택 | `POST /tumblebug/observability/stack`: VM 한 대에 nginx(TLS), OTel Collector 게이트웨이, Prometheus, Loki, Grafana와 대시보드 |
| 외부 엔드포인트 | `PUT /tumblebug/observability`: 임의의 OTLP/HTTP 엔드포인트, 선택적으로 Grafana |
| 에이전트 | OpenTelemetry Collector Contrib: 호스트 메트릭, journald/파일 로그, 선택적 Prometheus 스크레이프와 GPU 메트릭 |
| 식별자 | `TB_*`를 리소스 속성으로 붙여 Prometheus 라벨과 Loki 스트림 라벨에 `tb_infra`, `tb_node` 등이 실림 |
| 미러 | 스택이 `/dl/`에서 에이전트 패키지와 설치 스크립트를 제공하므로 노드는 GitHub에 접속하지 않음 |
| 계정 | Grafana 관리자(비밀은 `showSecrets=true`)와 reader API로 만드는 뷰어 계정 |

---

## 아키텍처

```
노드 (otelcol-contrib 에이전트) ──OTLP/HTTP 443, Bearer 토큰──▶ nginx ──▶ OTel Collector 게이트웨이 ──▶ Prometheus (remote write)
   hostmetrics, journald, filelog,              TLS                /v1/*        인증·배치                   Loki (OTLP 네이티브)
   prometheus (GPU 익스포터, 앱 포트)                               /dl/*  ──▶ 패키지 미러 + install-agent.sh
   resource: tb.ns/tb.infra/tb.nodegroup/tb.node, cloud.*          /      ──▶ Grafana (대시보드: Infra Overview, Node Detail, Node Logs, GPU, vLLM, Ollama)
```

- 관리형 스택은 443 포트 하나만 씁니다. `/`는 Grafana, `/v1/metrics`·`/v1/logs`는 OTLP 수신, `/dl/`은 에이전트 미러입니다. Security Group 템플릿 `sg-obs`는 443, 80(Let's Encrypt 검증용), 22를 엽니다.
- 게이트웨이가 리소스 속성을 Prometheus 라벨(`tb_infra`, `tb_node` 등)로 바꿉니다. Loki는 `tb.*`, `cloud.*`, `host.name`, `service.name`을 스트림 라벨로 색인합니다.
- 보존 기간은 두 저장소 모두 기본 15일입니다(스택 요청의 `retentionDays`).

---

## 스택 구성

**관리형(자체서명 인증서)**

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/stack -H 'Content-Type: application/json' -d '{}'
```

`system` 네임스페이스에 `infra-obs`(기본 `m5.2xlarge`, 500GB)를 만들고 전부 설치한 뒤 `https://<공인 IP>`를 스택 CA와 함께 등록합니다. 노드는 CA를 자동으로 받고, 브라우저에서는 Grafana 접속 시 인증서 경고가 뜹니다.

**DNS 이름과 Let's Encrypt로 관리형** (CB-Tumblebug에 등록된 Route53 호스티드 존 필요):

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/stack -H 'Content-Type: application/json' \
  -d '{"domainName": "example.com", "recordName": "obs", "retentionDays": 30}'
```

**기존 엔드포인트 등록**

```bash
curl -u default:default -X PUT http://localhost:1323/tumblebug/observability -H 'Content-Type: application/json' \
  -d '{"otlpEndpoint": "https://otel.example.com", "writeToken": "...", "grafanaUrl": "https://grafana.example.com", "grafanaAdminUser": "admin", "grafanaAdminPassword": "..."}'
```

Grafana 관리자 정보는 선택이며, 있으면 왕복 테스트가 저장까지 검증하고 뷰어 계정 API를 쓸 수 있습니다. 같은 값을 기동 시 `TB_OBSERVABILITY_*`로 줄 수도 있습니다(`conf/setup.env`, Helm `tumblebug.observability`).

**확인과 제거**

```bash
curl -u default:default http://localhost:1323/tumblebug/observability?showSecrets=true   # 토큰과 Grafana 관리자 비밀번호
curl -u default:default -X POST http://localhost:1323/tumblebug/observability/test       # 메트릭·로그 1건을 보내고 다시 조회
curl -u default:default -X DELETE 'http://localhost:1323/tumblebug/observability?deleteInfra=true'
```

---

## 노드에서 데이터 보내기

Infra(또는 NodeGroup) 요청에 `initscript-otel-agent` 템플릿을 넣습니다. `make init`이 `system` 네임스페이스에 적재합니다.

```json
"initScript": { "templateId": "initscript-otel-agent", "env": { "OTEL_INTERVAL": "30s" } }
```

템플릿은 작은 부트스트랩입니다. 스택에서 `install-agent.sh`를 받아 실행하고, 설치 스크립트는 스택 미러에서 Collector 패키지를 받아 노드 식별자가 들어간 설정을 쓰고 서비스를 켠 뒤 결과를 다른 init script 단계처럼 보고합니다(`initScriptStatus`). 데이터는 수집 주기 한 번 안에 Grafana에 나타납니다.

| Env | 기본값 | 의미 |
|-----|--------|------|
| `OTEL_METRICS` | `on` | 호스트 메트릭(CPU, 메모리, load, 디스크, 파일시스템, 네트워크, 페이징, 프로세스) |
| `OTEL_LOGS` | `on` | journald(있는 경우)와 init script 로그 `/var/log/tb-init-script.log` 전체 |
| `OTEL_GPU` | `auto` | `auto`는 스펙에 가속기가 있거나 `nvidia-smi`가 있으면 GPU 익스포터 설치, `on`/`off`는 강제 |
| `OTEL_INTERVAL` | `60s` | 수집·스크레이프 주기 |
| `OTEL_LOG_PATHS` | | 추가로 tail할 파일 glob(쉼표 구분), 예 `/var/log/nginx/*.log` |
| `OTEL_SCRAPE_TARGETS` | | 스크레이프할 Prometheus 엔드포인트 `host:port`(쉼표 구분). 에이전트가 스스로 찾지 못하는 엔드포인트에 사용 |
| `OTEL_DISCOVER` | `auto` | `auto`는 노드의 vLLM 서버를 찾아 메트릭 엔드포인트를 스크레이프(에이전트보다 나중에 뜬 서버 포함). `off`는 `OTEL_SCRAPE_TARGETS`에 적은 것만 스크레이프 |
| `OTEL_DIST` | `auto` | Collector 빌드. `auto`는 스택에 경량 빌드가 있고 스크레이프 대상이 없으면 경량 빌드, 아니면 `otelcol-contrib`. `trimmed`·`contrib`는 강제 |
| `OTEL_PACKAGE_URL` | 스택 `/dl` | 패키지 미러의 기본 URL. 대규모 구성에서 CDN 등을 앞에 둘 때 사용 |

**Collector 빌드.** 스택은 에이전트가 쓰는 구성요소만 넣은 경량 Collector `otelcol-tb`를 빌드해 둡니다. `otelcol-contrib`와 비교한 실측으로 다운로드는 114MB 대신 15MB, 노드의 메모리는 약 90MiB 대신 45MiB입니다. Prometheus 엔드포인트를 스크레이프하는 노드(GPU 익스포터, `OTEL_SCRAPE_TARGETS`, 에이전트가 찾은 vLLM 서버)는 Prometheus receiver가 있는 `otelcol-contrib`를 그대로 쓰고, 빌드 단계가 실패한 스택의 노드도 마찬가지입니다. 다운로드는 연결이 끊기거나 멈추면 다시 시도하고, 미러의 `SHA256SUMS`로 검증합니다.

**vLLM 자동 감지.** 에이전트는 노드에서 vLLM 서버(`vllm serve` 또는 `vllm.entrypoints.openai.api_server` 모듈)를 찾아, 주소를 알려 주지 않아도 메트릭 엔드포인트를 스크레이프합니다. 에이전트와 함께 설치되는 작은 서비스 `tb-otel-discover`(메모리 약 4MiB의 셸 반복문)가 1분마다 확인하고, 서버가 응답하기 시작하거나 프로세스가 끝나면 에이전트를 다시 적용합니다. 그래서 에이전트보다 한참 뒤에 설치한 서버도 응답을 시작한 뒤 1분 안팎이면 수집 대상에 들어가고, 모델을 로딩 중이거나 재시작 중인 서버는 그대로 유지됩니다. 경량 빌드를 쓰는 노드에서는 이때 Collector가 `otelcol-contrib`로 바뀌고, 서버가 없어지면 되돌아갑니다. 서버 인자에서는 포트만 읽습니다. 컨테이너 안의 서버는 같은 번호의 호스트 포트로 공개되어 있을 때 찾고, 그 밖의 엔드포인트는 `OTEL_SCRAPE_TARGETS`에 적습니다. 이 기능이 생기기 전에 에이전트를 설치한 노드는 템플릿을 SSH로 한 번 다시 실행하면 적용됩니다. 설치 때 `env`를 넣었다면 같은 값을 함께 넣습니다.

```bash
curl -u default:default -X POST 'http://localhost:1323/tumblebug/ns/default/cmd/infra/infra01?nodeId=g1-1' \
  -H 'Content-Type: application/json' -d '{"templateId": "initscript-otel-agent"}'
```

다른 단계와 평소처럼 조합할 수 있고, 첫 단계로 두면 뒤 단계가 소프트웨어를 설치하는 동안에도 에이전트가 이미 돌고 있습니다.

메모리가 1GiB 미만인 노드에서는 반대로 하세요. 에이전트가 떠 있는 상태에서 패키지를 설치하면 메모리가 바닥날 수 있습니다. 스왑이 없는 0.5GiB 노드에서 설치 단계가 10분 이상 멈췄고, 같은 단계를 순서만 바꾸면 1분 안에 끝났습니다. 설치 단계를 앞에 두거나 `initscript-swap` 템플릿으로 스왑을 추가하세요. 작은 스펙에서 에이전트 단계 뒤에 설치 단계가 있으면 Review가 경고합니다.

```json
"initScript": { "templateId": "initscript-otel-agent", "scripts": [ { "name": "app", "script": "..." } ] }
```

스택이 등록되어 있지 않으면 이 단계는 명확한 메시지(`TB_OTEL_ENDPOINT is empty`)로 실패하고 이후 단계가 중단되므로 스택을 먼저 등록하세요.

---

## 대시보드와 뷰어 계정

관리형 스택은 Grafana **CB-Tumblebug** 폴더에 대시보드 6개를 프로비저닝합니다(원본: `src/core/embedded/dashboards`).

- **TB Infra Overview** (`/d/tb-infra-overview`): 요약 지표(보고 중인 노드 수, CPU, 메모리, 루트 파일시스템, CPU당 load, 네트워크, 로그 줄 수, 저널 오류), CPU·메모리·load·프로세스·디스크 사용률/처리량/IOPS·네트워크 수신/송신·오류·TCP 연결의 상위 20개 노드 시계열, 색상 게이지가 있는 노드 표(노드 이름을 누르면 상세·로그 화면으로 이동). 네임스페이스·Infra·NodeGroup·노드로 필터.
- **TB Node Detail** (`/d/tb-node-detail`): 노드 한 대를 깊게 봅니다. 상태별 CPU, 상태별 메모리, CPU 수 대비 load, 상태별 프로세스, 마운트 지점별 파일시스템, 디스크별 사용률·대기열·처리량·IOPS, 인터페이스별 처리량·패킷·오류·드롭, 상태별 TCP 연결, 페이징, 그리고 그 노드의 저널과 init script 로그.
- **TB Node Logs** (`/d/tb-node-logs`): 노드별·레벨별(저널 priority를 error·warning·info·debug로 매핑) 분당 줄 수, 가장 시끄러운 유닛·식별자, 노드별 저널 오류, 그 아래에 `노드 식별자[pid]: 메시지` 형식의 저널과 init script·파일 로그. 텍스트·유닛 필터.
- **TB GPU** (`/d/tb-gpu`): GPU·노드 수, 평균 사용률, 유휴 GPU, 메모리 사용, 가장 뜨거운 GPU, 총 전력, 익스포터 실패를 타일로 보여 주고, GPU마다 한 줄씩 사용률·메모리 막대, 온도, 전력, 마지막 보고 시각을 표로 보여 줍니다. 추세는 사용률, 메모리, 온도, 전력(한계 대비), 부하 구간별 GPU 수입니다. `Per GPU` 행을 펼치면 GPU별 차트가, `Details` 행에는 익스포터 실패, 인코더·디코더, SM 클럭, 팬 속도가 있습니다. 표는 시간 범위 안의 마지막 값을 보여 주므로 보고를 멈춘 GPU도 남아 있습니다.
- **TB vLLM** (`/d/tb-vllm`): 보고 중인 서버, 실행·대기 중 요청, 초당 요청·출력 토큰, 첫 토큰 시간, KV 캐시 사용, 실패한 요청을 타일로, 서버·모델별 한 줄 표, 요청·토큰·지연 백분위·요청 시간 구성·프리픽스 캐시 추세를 보여 줍니다. vLLM은 API 포트에서 Prometheus 메트릭을 제공하고, 에이전트가 서버를 스스로 찾습니다(위의 vLLM 자동 감지).
- **TB Ollama** (`/d/tb-ollama`): 요청 수, 실패 비율, 요청 시간, 생성 속도, 출력 토큰, 모델 로드를 `ollama.service`의 저널 로그에서 읽어 보여 줍니다. Ollama에는 메트릭 엔드포인트가 없어 스크레이프할 것이 없고, Ollama가 systemd 서비스(기본 설치 방식)로 떠 있으면 에이전트의 저널 로그만으로 충분합니다. 접근 로그에 모델 이름이 없어 모델별로 나누지는 못합니다.

Grafana가 있는 스택이 등록되어 있으면 MapUI의 Infra Status 카드에 **📈 Metrics**, **📜 Logs** 링크가 나타나며 해당 Infra가 선택된 대시보드를 엽니다.

뷰어 계정(`POST /tumblebug/observability/reader`)은 Viewer 역할의 Grafana 사용자이고 비밀번호는 한 번만 반환됩니다. MapUI의 Resource Provisioning 메뉴 **Observability (OpenTelemetry)**에 같은 폼이 있습니다.

---

## GPU 메트릭

노드 스펙에 가속기(`acceleratorType`)가 있으면 CB-Tumblebug이 `TB_ACCELERATOR_TYPE`, `TB_ACCELERATOR_MODEL`, `TB_ACCELERATOR_COUNT`를 init script에 내려줍니다. `OTEL_GPU=auto`면 설치 스크립트가 nvidia-smi 기반 익스포터(스택 미러 제공)를 설치하고 `127.0.0.1:9835` 스크레이프를 추가합니다. 드라이버가 아직 없어도 익스포터는 설치되며 `nvidia-smi`가 동작하는 순간부터 메트릭이 흐르므로, 나중에 SSH(post-deployment 명령)로 드라이버를 설치해도 됩니다. DCGM 수준 지표(NVLink, ECC, MIG)는 이 익스포터가 다루지 않습니다.

---

## API 레퍼런스

모든 엔드포인트는 전역(네임스페이스 없음)이며 CB-Tumblebug API 인증이 필요합니다.

| 메서드 | 경로 | 설명 |
|--------|------|------|
| `GET` | `/tumblebug/observability` | 등록된 스택(없으면 204). `showSecrets=true`가 아니면 비밀 마스킹 |
| `PUT` | `/tumblebug/observability` | 기존 OTLP 엔드포인트 등록(`otlpEndpoint`, `writeToken?`, `caCert?`, `grafanaUrl?`, `grafanaAdminUser?`, `grafanaAdminPassword?`). URL은 `http(s)://host[:port][/path]` 형식(자격증명·query·fragment 불가, 링크로컬·메타데이터 주소 거부) |
| `DELETE` | `/tumblebug/observability` | 등록 해제. `deleteInfra=true`면 관리형 Infra와 자원도 삭제 |
| `POST` | `/tumblebug/observability/stack` | 템플릿으로 프로비저닝 후 등록. 본문: `nsId`, `templateId`, `name`, `domainName`, `recordName`, `retentionDays`(모두 선택) |
| `POST` | `/tumblebug/observability/test` | 테스트 메트릭·로그 전송, Grafana 관리자 정보가 있으면 조회까지 검증(`timeoutSec`) |
| `POST` | `/tumblebug/observability/reader` | Grafana 뷰어 생성 `{name, password?}` |
| `GET` | `/tumblebug/observability/reader` | 이 API로 만든 뷰어 목록 |
| `DELETE` | `/tumblebug/observability/reader/{name}` | 뷰어 삭제 |

---

## 규모·보안·제약

- **규모.** 60초 주기 호스트 메트릭은 노드당 수백 시리즈입니다. 기본 스택(8 vCPU, 32GB, 500GB)은 수천 노드 규모를 염두에 둔 크기이며, 수만 노드면 주기를 늘리거나 VM 메모리를 키우거나 게이트웨이 뒤 저장소를 더 큰 것으로 바꾸면 됩니다. 에이전트는 엔드포인트와 토큰만 알기 때문에 노드를 건드리지 않고 백엔드를 교체할 수 있습니다. 패키지는 스택 미러에서 받으므로 대규모 롤아웃이 GitHub에 의존하지 않습니다.
- **보안.** 수신은 bearer 토큰이 필요하고 Grafana는 로그인이 필요하며 열린 포트는 443과 22뿐입니다. 자체서명 스택의 CA는 CB-Tumblebug에 저장되어 노드에 전달됩니다. `showSecrets=true`는 API 사용자에게 토큰과 Grafana 관리자 비밀번호를 노출합니다.
- **네트워크.** 노드는 스택으로의 아웃바운드 HTTPS가, 스택은 설치 중 인터넷(GitHub 릴리스, apt.grafana.com)이 필요합니다.
- **Linux 전용**입니다(init script와 동일). 에이전트는 OpenTelemetry Collector Contrib 패키지(amd64·arm64, deb·rpm)입니다.
- 제거된 CB-Dragonfly 메트릭을 쓰던 **오토스케일링 정책**은 아직 이 스택과 연결되지 않아 "no metric source"로 실패합니다.
