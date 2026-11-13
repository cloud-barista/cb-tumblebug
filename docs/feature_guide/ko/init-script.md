# 초기화 스크립트 (Init Script, 첫 부팅 프로비저닝)

`initScript` 가이드: CB-Tumblebug이 모든 Linux 노드에 cloud-init user data로 전달하는 첫 부팅 스크립트입니다. SSH 계정 생성, 재사용 가능한 스크립트 템플릿, 순차 실행 단계, [Notifier](notifier-ntfy.md)를 통한 결과 보고를 다룹니다.

## 📑 목차

1. [개요](#개요)
2. [동작 방식](#동작-방식)
3. [요청 형식](#요청-형식)
4. [스크립트 템플릿](#스크립트-템플릿)
5. [스크립트에서 쓸 수 있는 변수](#스크립트에서-쓸-수-있는-변수)
6. [검토와 결과 확인](#검토와-결과-확인)
7. [API 레퍼런스](#api-레퍼런스)
8. [제약 사항과 문제 해결](#제약-사항과-문제-해결)

---

## 개요

### Init script란?

모든 CSP는 VM 첫 부팅 때 user data를 한 번 실행합니다(대부분 cloud-init). 지금까지는 CB-Spider가 이 user data를 만들어 `cb-user` 계정과 SSH 키를 넣었습니다. `initScript`를 쓰면 CB-Tumblebug이 user data를 직접 렌더링해 CB-Spider에 `UserData`로 넘기고, 노드는 다음을 수행합니다.

1. Infra의 키로 SSH 계정(기본 `cb-user`)과 비밀번호 없는 sudo를 만들고,
2. 사용자 스크립트를 순서대로 실행하다 첫 실패에서 멈추고,
3. 각 단계가 시작될 때와 마지막에 Notifier로 보고합니다. CB-Tumblebug은 가장 최근 스냅샷을 노드의 `initScriptStatus`(`pending` → `initializing` → `ok`/`fail`)로 기록합니다.

### 왜 이 기능을 사용하는가?

- **대규모에서 SSH가 필요 없습니다.** 노드 1만 대를 `postCommands`로 부트스트랩하면 CB-Tumblebug이 SSH 세션 1만 개를 열어야 합니다. Init script는 VM이 스스로 실행하고 결과를 보고하므로 CB-Tumblebug이 사설망에 있어도 됩니다.
- **모든 CSP에서 같은 동작.** AWS, Azure, GCP, Alibaba, Tencent, IBM, OpenStack, KT, NCP, NHN에서 같은 래퍼가 실행됩니다.
- **단계 재사용.** 스크립트를 `system` 네임스페이스(공용) 또는 사용자 네임스페이스에 템플릿으로 저장하고, 기본 하드닝 → 모니터링 에이전트 → 역할별 스크립트처럼 순서대로 조합할 수 있습니다.

### 핵심 특징

| 기능 | 설명 |
|------|------|
| 모드 | `default`(계정 + 스크립트, `initScript`를 생략해도 이 값), `custom`(스크립트만, 이미지 기본 계정 유지), `none`(CB-Spider 기본) |
| 단계 | 템플릿의 스크립트들 → `script` → `scripts[]` 순서로 실행, 0이 아닌 종료 코드가 나오면 중단 |
| 템플릿 | `/ns/{nsId}/template/initScript`: 순서 있는 스크립트 목록 + 기본 env; 요청 네임스페이스에서 먼저 찾고 없으면 `system` |
| Env | 템플릿 `env` 기본값 위에 요청 `env`를 덮어써 모든 단계에 export |
| 결과 | 노드의 `initScriptStatus`(`pending`/`initializing`/`ok`/`fail`)와 `initScriptMessage`, 집계와 단계별 상세는 `GET /ns/{nsId}/infra/{infraId}/initScript` |
| 보고 | Notifier가 등록되어 있으면 기본 보고, `notify: false`로 NodeGroup 단위로 끌 수 있음 |
| 크기 제한 | 렌더링된 user data 16KB(AWS 제한; Review API가 검사) |

---

## 동작 방식

```
Infra / NodeGroup 요청 ──▶ 템플릿 해석 ──▶ user data 렌더링 ──▶ CB-Spider (UserData)
      initScript           (NodeGroup 블루프린트에   #!/bin/bash                │
                            복사되어 저장)           export TB_*=…              ▼
                                                     래퍼                   VM 첫 부팅
                                                                                │
                                        Node.initScriptStatus ◀── Notifier ◀── 보고 (ntfy)
```

- 템플릿은 Infra 또는 NodeGroup 생성 시점에 해석되고, 결과 단계가 NodeGroup에 저장됩니다. 이후 scale-out으로 추가되는 노드는 템플릿이 바뀌어도 같은 내용을 받습니다.
- 래퍼는 바이너리에 내장됩니다(`src/core/embedded/scripts/init-script.sh`). 상태 마커 `/var/lib/tb/init-script.status`가 있으면 즉시 종료하므로, 매 부팅마다 user data를 실행하는 CSP(GCP)에서도 한 번만 동작합니다.
- 단계 출력은 노드의 `/var/log/tb-init-script.log`에 쌓이고, 각 단계는 `/var/lib/tb/init-script-<n>.sh`로 저장됩니다.
- 요청에 `initScript`가 없으면 CB-Tumblebug이 기본값(`mode: default`, `cb-user`, 단계 없음)을 적용합니다. 모든 Linux 노드가 같은 방식으로 계정을 만들고 부팅 완료를 보고하게 됩니다. 이런 암묵적 요청에서 user data를 만들 수 없으면(예: SSH 공개키를 구할 수 없음) 노드는 CB-Spider 기본 user data로 생성되고 `systemMessage`에 그 사실이 남습니다. 명시적으로 지정한 `initScript`는 그런 경우 노드를 실패시킵니다.
- 보고 결과는 노드 레코드를 갱신하지 않고 노드별 전용 레코드에 한 번의 쓰기로 저장되므로, 수천 대가 동시에 보고해도 프로비저닝이 느려지지 않습니다.
- Windows 노드와 `mode: none`은 CB-Spider 기본 user data를 그대로 사용합니다.

---

## 요청 형식

`initScript`는 Infra 요청(모든 NodeGroup에 적용)에 두고 NodeGroup별로 덮어쓸 수 있습니다. `POST /ns/{nsId}/infraDynamic`, `POST /ns/{nsId}/infraDynamicReview`, `POST /ns/{nsId}/infra/{infraId}/nodeGroupDynamic`, Infra 템플릿에서 사용합니다.

```json
{
  "name": "web",
  "initScript": {
    "mode": "default",
    "userName": "cb-user",
    "templateId": "initscript-nginx",
    "scripts": [
      { "name": "role", "script": "#!/bin/bash\necho web > /etc/role\n" }
    ],
    "env": { "ROLE": "web", "OTEL_ENDPOINT": "https://otel.example.com" }
  },
  "nodeGroups": [ { "name": "g1", "specId": "aws+ap-northeast-2+t3.small", "nodeGroupSize": 2 } ]
}
```

| 필드 | 설명 |
|------|------|
| `mode` | `default`는 계정을 만들고 단계를 실행합니다. `custom`은 단계만 실행하며, 계정이 이미 있는 이미지에서 사용합니다(이때 노드의 `nodeUserName`을 그 계정으로 지정). `none`은 기능을 끕니다. |
| `userName` | `default` 모드에서 만들 계정(기본 `cb-user`). |
| `templateId` | 스크립트들을 먼저 순서대로 실행할 init script 템플릿. 요청당 하나이며, 템플릿의 기본 `env` 위에 요청 `env`가 덮어씁니다. |
| `script` | `inline`이라는 이름의 인라인 단계 축약 표기. |
| `scripts[]` | 템플릿 단계 뒤에 이어 붙는 인라인 단계 `{name?, script, continueOnError?, timeoutSec?, labelSelector?}`. 이름이 없으면 `step-<n>`. |
| `env` | 모든 단계에 export되는 변수. 키는 셸 변수 이름 규칙을 따르고 `TB_`로 시작할 수 없으며, 값에 작은따옴표와 줄바꿈을 쓸 수 없습니다. |
| `notify` | `false`면 노드가 Notifier로 보고하지 않습니다. 기본은 Notifier가 등록되어 있을 때 보고. |

단계는 템플릿 스크립트, `script`, `scripts[]` 순서로 실행됩니다. 각 단계는 `bash`로 실행되므로 셔뱅은 없어도 됩니다.

단계별 옵션(템플릿 안에서도 사용 가능):

| 옵션 | 효과 |
|------|------|
| `continueOnError` | 이 단계가 실패해도 계속 진행합니다. 노드는 `ok`로 보고되고 `initScriptMessage`에 `optional step(s) failed: ...`가, 단계 목록에는 `fail`이 남습니다 |
| `timeoutSec` | 지정한 초가 지나면 단계를 중단(종료 코드 124)하고 실패로 처리합니다 |
| `labelSelector` | 라벨이 맞는 노드에서만 실행합니다(post-deployment 명령과 같은 문법, 예 `role=gpu`). 역할이 다른 그룹을 Infra 단위 스크립트 하나로 다룰 수 있습니다 |

**Infra 단위와 NodeGroup 단위.** Infra의 `initScript`는 모든 NodeGroup에 적용됩니다. `mode`, `userName`, `notify`를 지정하지 않은 NodeGroup `initScript`는 Infra 것을 *확장*합니다(그룹의 `templateId`, `script`, `scripts`가 Infra 단계 뒤에 실행되고 env는 덮어씀). 셋 중 하나라도 지정하면 그 그룹에서는 Infra 것을 *대체*합니다. 템플릿의 단계 이름은 템플릿이 지정하지 않으면 템플릿 id(`initscript-nginx`, 여러 단계면 `initscript-nginx-2`)가 됩니다.

---

## 스크립트 템플릿

Init script 템플릿은 순서 있는 스크립트 목록(`scripts[]`, 하나면 `script`)과 기본 `env`를 저장합니다. 기본 설정 → 에이전트 설치 → 애플리케이션 시작처럼 하나의 프로필을 템플릿 하나로 표현할 수 있습니다. `system` 네임스페이스의 템플릿은 모든 네임스페이스에서 공용으로 쓰이며, 요청 네임스페이스에 같은 id의 템플릿이 있으면 그쪽이 우선합니다. `make init`은 `init/templates/initscript-*.json`을 `system`에 적재합니다. 기본 제공은 `initscript-hello`(마커 기록)와 `initscript-nginx`(nginx 설치 후 노드를 식별하는 페이지 제공)이며, nginx는 80 포트가 열리도록 `infra-usecase-aws-ap-northeast-2-web` 템플릿처럼 `sgTemplateId: sg-usecase-web`과 함께 씁니다. `initscript-swap`은 메모리가 1GiB 미만인 노드에 스왑 파일을 추가해, 서비스가 떠 있는 상태의 패키지 설치로 노드가 멈추지 않게 합니다.

```bash
# system 네임스페이스에 공용 템플릿 생성
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/system/template/initScript \
  -H 'Content-Type: application/json' -d '{
    "name": "web-profile",
    "description": "nginx와 OpenTelemetry collector",
    "scripts": [
      { "name": "nginx", "script": "#!/bin/bash\napt-get install -y -qq nginx\n" },
      { "name": "otel-agent", "script": "#!/bin/bash\ncurl -fsSL https://example.com/otel.sh | OTEL_ENDPOINT=$OTEL_ENDPOINT bash\n" }
    ],
    "env": { "OTEL_ENDPOINT": "https://otel.example.com" }
  }'

# initscript-hello를 참조하는 across-global 템플릿으로 Infra 생성
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/default/infra/template/infra-across-global \
  -H 'Content-Type: application/json' -d '{"name": "demo"}'
```

템플릿을 수정하면 그 이후에 만드는 Infra와 NodeGroup에만 반영됩니다. 템플릿 id는 일반 리소스 id 규칙(소문자, 숫자, `-`)을 따릅니다.

---

## 스크립트에서 쓸 수 있는 변수

| 변수 | 의미 |
|------|------|
| `TB_NS_ID`, `TB_INFRA_ID`, `TB_NODEGROUP_ID`, `TB_NODE_ID` | CB-Tumblebug에서의 노드 식별자 |
| `TB_CSP`, `TB_REGION` | 노드의 CSP와 리전 |
| `TB_INIT_USER` | 이 init script의 계정 이름(`userName`을 지정하지 않으면 `cb-user`) |
| `TB_NOTIFY_URL`, `TB_NOTIFY_TOPIC` | 보고 대상 Notifier(등록된 경우에만 존재) |
| `TB_OTEL_ENDPOINT`, `TB_OTEL_TOKEN`, `TB_OTEL_CA_B64` | [관측성 스택](observability-otel.md)(등록된 경우에만 존재) |
| `TB_ACCELERATOR_TYPE`, `TB_ACCELERATOR_MODEL`, `TB_ACCELERATOR_COUNT` | 노드 스펙의 가속기(가속기가 있는 스펙에서만 존재, 예 `gpu`) |
| 사용자 `env` | 템플릿과 요청 `env`를 합친 모든 키 |

`TB_`로 시작하는 변수는 예약되어 있어 요청 `env`로 설정할 수 없습니다.

---

## 검토와 결과 확인

**생성 전 검토.** `POST /ns/{nsId}/infraDynamicReview`는 템플릿을 해석하고 자리표시자 값으로 user data를 렌더링합니다. 노드 검토마다 모드·계정·단계·대략적 크기를 담은 `info` 줄이 붙고, 템플릿이 없거나 user data가 16KB를 넘으면 `error`, Notifier가 등록되어 있지 않으면 `warning`(노드가 보고할 수 없음)이 붙습니다.

**노드의 결과.** 보고가 켜진 init script를 가진 노드는 첫 보고가 도착할 때까지 `initScriptStatus: pending`으로 보입니다. 단계가 실행되는 동안은 `initializing`이고 `initScriptMessage`에 `initializing: initscript-nginx (1/2)`처럼 현재 단계가 표시됩니다. VM이 끝나면 `ok` 또는 `fail`이 되고, 실패 시 `initScriptMessage`는 실패한 단계 이름과 종료 코드로 시작하고 로그 마지막 줄이 이어집니다.

```json
{ "id": "g1-1", "initScriptStatus": "fail", "initScriptMessage": "role exit 1: Job for nginx.service failed ..." }
```

**목록 조회.** MapUI가 지도용으로 폴링하는 `GET /ns/{nsId}/infra` 응답의 각 노드에도 `initScriptStatus`, `initScriptMessage`와, 초기화 중이거나 실패한 경우에만 `initScriptSteps`가 실립니다. 보고 결과의 메모리 인덱스에서 읽으므로 큰 Infra를 폴링해도 etcd 읽기가 늘지 않습니다.

**Infra 전체 한 번에.** `GET /ns/{nsId}/infra/{infraId}/initScript`는 노드 레코드를 읽지 않고 집계와 노드별 결과(단계별 상세 포함)를 돌려줍니다. `option=summary`는 집계만, `status=ok|fail|pending|notApplicable`은 해당 상태만 나열합니다. 보고가 아직 없는 노드는 `pending`이고, Running이 된 뒤 `TB_INIT_SCRIPT_NO_REPORT_TIMEOUT`(기본 `3m`)이 지나도 보고가 없으면 `noReport`(user data 미실행, 또는 그 리전에서 Notifier에 닿지 못함. 늦게라도 보고가 오면 덮어씁니다), NodeGroup이 `mode: none`이거나 노드가 Running에 이르지 못했으면(Failed·Terminated) `notApplicable`입니다.

```json
{ "infraId": "web", "total": 100, "notApplicable": 0, "reported": 98, "ok": 97, "fail": 1, "pending": 2,
  "nodes": [
    { "nodeId": "g1-1", "nodeGroupId": "g1", "status": "fail", "exitCode": 1, "durationSec": 42, "reportedAt": "2026-01-01T00:00:10Z",
      "steps": [ { "name": "initscript-hello", "status": "ok", "exitCode": 0, "durationSec": 0 },
                 { "name": "role", "status": "fail", "exitCode": 1, "durationSec": 41 } ],
      "message": "..." },
    { "nodeId": "g1-2", "nodeGroupId": "g1", "status": "pending", "exitCode": 0, "durationSec": 0 } ] }
```

VM 안에서는 `/var/lib/tb/init-script.status`에 `ok|fail <종료 코드> <시각>`이, `/var/log/tb-init-script.log`에 전체 출력이 남습니다.

---

## API 레퍼런스

| 메서드 | 경로 | 설명 |
|--------|------|------|
| `POST` | `/ns/{nsId}/template/initScript` | 템플릿 생성 `{name, description?, scripts[] 또는 script, env?}` |
| `GET` | `/ns/{nsId}/template/initScript` | 네임스페이스의 템플릿 목록(`filterKeyword` 선택) |
| `GET` | `/ns/{nsId}/template/initScript/{templateId}` | 템플릿 조회 |
| `PUT` | `/ns/{nsId}/template/initScript/{templateId}` | 스크립트 목록·env·설명 교체 |
| `DELETE` | `/ns/{nsId}/template/initScript/{templateId}` | 템플릿 삭제 |
| `DELETE` | `/ns/{nsId}/template/initScript` | 네임스페이스의 템플릿 전체 삭제 |

| `GET` | `/ns/{nsId}/infra/{infraId}/initScript` | 집계와 노드별 결과(`option=summary`, `status=` 필터) |

`initScript` 자체는 Infra, Infra 템플릿, NodeGroup 동적 요청의 필드이며, 노드 객체는 `initScript`, `initScriptStatus`, `initScriptMessage`를 노출합니다.

**나중에 SSH로 템플릿 실행.** 원격 명령(`POST /ns/{nsId}/cmd/infra/{infraId}`, `nodeGroupId`·`nodeId`·`labelSelector` 지정)과 post-deployment 단계는 `command` 대신 `templateId`와 `scripts`, 그리고 `env`를 받을 수 있습니다. 단계는 첫 부팅 때와 같은 `TB_*` 변수와 규칙으로 root로 실행됩니다. 기존 노드에 템플릿을 적용하거나 실패한 init script를 노드 재생성 없이 다시 돌릴 때 씁니다.

```bash
curl -u default:default -X POST http://localhost:1323/tumblebug/ns/default/cmd/infra/web?nodeId=g1-1 \
  -H 'Content-Type: application/json' -d '{"templateId": "initscript-nginx", "env": {"SITE_TITLE": "retry"}}'
```

이렇게 실행하면 첫 부팅과 같이 Notifier로 `event: done`을 보고하므로 노드의 `initScriptStatus`와 단계 결과는 가장 최근 실행을 따릅니다. MapUI에서는 init이 실패한 노드의 Infra 카드에 **re-run via SSH** 버튼이 보이며 그 노드의 단계를 그대로 다시 실행합니다. post-deployment 명령 작성기에서는 단계마다 명령 대신 템플릿을 고를 수 있고, 검토 창에서는 NodeGroup별로 템플릿·인라인 단계를 추가할 수 있습니다.

일반 `command` 요청도 `env`를 주면 각 명령 앞에 export됩니다.

---

## 제약 사항과 문제 해결

- **Linux 전용.** Windows 노드는 CB-Spider 기본(CSP를 통한 계정·비밀번호)을 유지합니다.
- **16KB 제한.** 변수, 래퍼, base64로 인코딩된 모든 단계를 합친 user data가 16KB 미만이어야 합니다. 큰 내용은 인라인하지 말고 다운로드 단계로 분리하세요.
- **Notifier가 없으면 보고되지 않습니다.** 스크립트는 실행되지만 노드는 `pending`으로 남습니다. 이때는 VM의 상태 마커가 기준입니다.
- **전체 로그.** 보고에는 마지막 줄만 담깁니다. 노드의 `/var/log/tb-init-script.log`에 전체가 있고, 관측성 스택을 쓰면 OTel 에이전트가 이 파일을 그대로 올립니다.
- **`env`의 비밀값.** CB-Tumblebug은 스크립트 본문, `env` 값, 렌더링된 user data를 자체 로그, 디버그 출력, 원격 명령 이력에 남기지 않습니다. `[N bytes redacted]` 또는 `********`로 표시됩니다. 다만 노드 블루프린트에는 저장되어 인증된 API 사용자에게 보이고, VM 안에서는 user data가 평문입니다. 수명이 긴 비밀값은 `env`에 넣지 마세요.
- **보고는 최선 노력 방식입니다.** 진행 보고(단계 시작마다 1회)는 한 번만 보내고, 최종 보고는 3회 재시도합니다. VM 네트워크에서 Notifier에 닿지 않으면 Notifier VM의 Security Group(443)과 노드의 아웃바운드를 확인하세요.
- **`custom` 모드와 SSH.** `custom` 모드에서는 계정을 만들지 않으므로 원격 명령은 이미지 기본 계정을 사용합니다. `nodeUserName`을 그에 맞게 지정하세요.
- **마커가 이미 있는 경우.** Init script가 실행된 노드로 만든 커스텀 이미지에는 `/var/lib/tb/init-script.status`가 들어 있어 새 VM에서 래퍼가 바로 종료됩니다. 이미지를 만들기 전에 파일을 지우세요.
