# v4.0 documentation validation

Input: Drevanta-Consolidated-Specification-and-Approval-v3.1(1).zip  
SHA-256: ee27e3813542c35f0540b66dd2c8793a9cbe5811befd25a038f810e8d705e13d

| Check | Observed result |
| --- | --- |
| Preserved specification | 25 phase documents byte-identical, with v4.0 profiles and registers |
| Prototype provenance | 71 source files matched size and SHA-256 |
| Scope and source | 404 overlapping scope records, including 47 seeds and 18 workflows; 339 numbered sections, 4061 source lines, 421 source-ID anchors |
| Static UI and authorization | 31 screens, 239 inherited controls, 85 parsed HTML elements including utility elements, 40 role/policy references |
| Contracts and tests | 178 historic method/path candidates; 744 inherited cases and 17 domain fixtures; references resolve |
| Execution | 14 ordered nodes covering 25 phases, scope entries and test IDs |
| Browser and runtime | Browser launch failed before navigation; no runtime tests executed |
| Archive | Manifest digest, CRC and readback validated |

The static census does not establish semantic completeness, effective permission coverage, or executable bindings for every case. No GitHub, credential, VPS, implementation or deployment activity occurred.
