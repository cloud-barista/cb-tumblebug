# Usecase of Elastic Stack

이 README 파일은 Elastic Stack (ELK Stack 및 Filebeat)의 기본적인 설치 및 관리 절차를 안내합니다. 
사용자는 이 문서를 바탕으로 Elastic Stack 환경을 구축하고, 필요에 따라 추가 설정을 진행할 수 있을 것 입니다.

## Elasticsearch, Logstash, Kibana, Filebeat 설치 및 활용을 위한 스크립트

### 소개
Elastic Stack은 로그 및 데이터 분석 파이프라인에서 중요한 역할을 합니다. 

각 컴포넌트의 역할은 다음과 같습니다.
- **Elasticsearch**: 데이터 저장, 검색 및 분석
- **Kibana**: 데이터 탐색, 시각화 및 공유
- **Logstash**: 데이터 수집, 풍부화 및 전송
- **Beats**: 데이터 수집, 파싱 및 전송

이해를 돕기위해 상호 연관성을 조금만 설명하고 넘어가겠습니다.
- **Beats (예: Filebeat)**: 파일에 저장된 로그를 수집하여 Logstash로 전송합니다.
- **Logstash**: 수집된 로그 데이터를 처리하고 Elasticsearch로 전송합니다.
- **Elasticsearch**: 처리된 로그 데이터를 저장, 검색 및 분석합니다.
- **Kibana**: Elasticsearch에 저장된 데이터를 읽고 시각화합니다.

### Elasticsearch, Logstash, Kibana (ELK Stack) 및 Filebeat 설치 및 관리

작성된 스크립트를 바탕으로 ELK Stack과 Filebeat의 설치, 설정, 실행, 상태 조회, 중지 및 삭제에 대한 기본적인 절차를 설명합니다.

#### ELK Stack

- **ELK Stack 시작**: 
  ```bash
  ./startELK.sh
  ```

- **ELK Stack 상태 조회**: 
  ```bash
  ./statusELK.sh
  ```

- **ELK Stack 중지**: 
  ```bash
  ./stopELK.sh
  ```

- **ELK Stack 삭제**: 
  ```bash
  ./removeELK.sh
  ```

#### Filebeat
- **Filebeat 시작**:
  ```bash
  ./startFilebeat.sh
  ```

- **Filebeat 상태 조회**:
  ```bash
  ./statusFilebeat.sh
  ```
  
- **Filebeat 중지**:
  ```bash
  ./stopFilebeat.sh
  ```

- **Filebeat 삭제**:
  ```bash
  ./removeFilebeat.sh
  ```

### 마치며
- 위 스크립트는 ELK Stack과 Filebeat의 기본적인 관리를 위한 것입니다. 
- 각 스크립트의 상세 내용은 스크립트 파일 내부를 참조하십시오.
- ELK Stack과 Filebeat의 보다 상세한 설정 및 관리 방법은 각 공식 문서를 참조하시기 바랍니다.
