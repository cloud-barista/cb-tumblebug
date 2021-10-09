# Contributing

CB-Tumblebug welcomes improvements from both new and experienced contributors!

### (1) Contribution Types

- Open Issue
  - Bug report, Enhancement request, Feature request, ...
- Open PR (Pull-Request) 
  - Documentation, Source code, ...  
- Not limited ..

### (2) Contribution Guide

- In general
  - [Cloud-Barista Contribution Overview](https://github.com/cloud-barista/docs/blob/master/CONTRIBUTING.md#how-to-contribute)
  - [Cloud-Barista Code of Conduct](https://github.com/cloud-barista/docs/blob/master/contributing/CODE_OF_CONDUCT.md)
- In detail
  - [How to open and update a PR](https://github.com/cloud-barista/docs/blob/master/contributing/how_to_open_a_pull_request-ko.md)
    - **Be careful!** 
      - Contributors should not push files related to their personal credentials (e.g., credentials.conf) to remote repository.
      - The credential file for CSPs (`src/testclient/scripts/credentials.conf`) is in the [.gitignore](https://github.com/cloud-barista/cb-tumblebug/blob/ed250835a1357784afd4c857d6bd56e0d78cd219/.gitignore#L36) condition.
      - So, `credentials.conf` will not be staged for a commit.
      - Anyway, please be careful.
  - [Test requirement for developers](https://github.com/cloud-barista/cb-tumblebug/wiki/Basic-testing-guide-before-a-contribution)
  - [Coding convention for developers](https://github.com/cloud-barista/cb-tumblebug/wiki/Coding-Convention)
