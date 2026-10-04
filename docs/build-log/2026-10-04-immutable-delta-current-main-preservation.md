# Immutable delta current-main preservation

Same original BASE/key/enrollment. Exact absent-at-BASE tip copies are omitted under the reviewed preservation disposition; original raw bindings and seals remain in ordinary ancestry.

```json
{
  "originalBase": "79bd635d91f0f3944c781882ae6f6d4bb968d58b",
  "originalKey": "61668a08d33c7865786bc2f8fb8cf982997d7072c78fae7ad8a81d9f92c261cd",
  "originalPlanDigest": "94aa3939437eed96da6955440485e861e8fe7489a9804b937ddef7742d026fea",
  "originalSeal": "81ea7808b102b09ed79111fe3265c90687e00fcd",
  "actualMain": "6c31bac621fa0d3d81228a7c4b67e8e9466a579a",
  "baseArchiveTableSha256": "e2157733d7cfdb1a7c4cf2f2c78936b83d46595bb95309f147a283d7e64e339c",
  "allBaseArchivesExact": 350,
  "historicalBindings": [
    {
      "path": ".corvint/changes/0ab7b241bfda371ca63c2238a27d823d8507e724.cem.json",
      "mode": "100644",
      "blob": "9a68a8d06ef30e4a248f049845ae665d80c97c64",
      "sha256": "6ebe9c18304a3697747997d97de4c5545583ca2115581daf3be9a53f48582396",
      "bind": "0ab7b241bfda371ca63c2238a27d823d8507e724",
      "seal": "56f638d62070a982250e709ea75674e1ca5862dc",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/0ab7b241bfda371ca63c2238a27d823d8507e724.cem.json"
    },
    {
      "path": ".corvint/changes/1722ce9354bbc7d6f630e627eba5c4c1e7459301.cem.json",
      "mode": "100644",
      "blob": "a38d4b5eab888bddc80b66fcaef893e0f8898078",
      "sha256": "aa4235343339bd7637cbf141be518343137b983b4275250881b2be857a472d8e",
      "bind": "1722ce9354bbc7d6f630e627eba5c4c1e7459301",
      "seal": "e2fa015588ae5263d4381a26b241cd1f2d2e7056",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/1722ce9354bbc7d6f630e627eba5c4c1e7459301.cem.json"
    },
    {
      "path": ".corvint/changes/469969beaed20f72228525bead0b57ab9766ee85.cem.json",
      "mode": "100644",
      "blob": "57ab7968423d4775cd70008e72fbf0740ab6149e",
      "sha256": "01f1d27210f86ab1a816dbeae7fd2b5e8ea5dda10c985c5f17f4ee3d6d4abd76",
      "bind": "469969beaed20f72228525bead0b57ab9766ee85",
      "seal": "81ea7808b102b09ed79111fe3265c90687e00fcd",
      "sourceParent": "81ea7808b102b09ed79111fe3265c90687e00fcd",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/469969beaed20f72228525bead0b57ab9766ee85.cem.json"
    },
    {
      "path": ".corvint/changes/9025991ee0413502897fec7827cdf855c62a9f10.cem.json",
      "mode": "100644",
      "blob": "102e599bb9353e587630368d527986568897a700",
      "sha256": "147da6909fa055dda61536cf3f66da300455d6083bf5abbf5d23f88912b3a2df",
      "bind": "9025991ee0413502897fec7827cdf855c62a9f10",
      "seal": "21a6f77b155849f0f6d52340c7ff66e465fee936",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/9025991ee0413502897fec7827cdf855c62a9f10.cem.json"
    },
    {
      "path": ".corvint/changes/bee1cf5c75bc640b1c4e2d501b995aaf207cc501.cem.json",
      "mode": "100644",
      "blob": "4a100e8927c330a52950f7160d025e89dc4e01c7",
      "sha256": "dc3596d17a221435ecec46a758b95d6f0571ddc6d86daf29cbb61108b7b2fb55",
      "bind": "bee1cf5c75bc640b1c4e2d501b995aaf207cc501",
      "seal": "9ef921a1d5af2fcdaab1ff05b54a4bce05115d38",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/bee1cf5c75bc640b1c4e2d501b995aaf207cc501.cem.json"
    },
    {
      "path": ".corvint/changes/cd3a19d8a2492560c40ea10d453620dbd2ec5c55.cem.json",
      "mode": "100644",
      "blob": "bd750cb4d07d16dd43891b58abb674be228bb4a2",
      "sha256": "36aa7b95ba8e60f20e89845e7131a97afedcacadd652a9bbc722c49c05063690",
      "bind": "cd3a19d8a2492560c40ea10d453620dbd2ec5c55",
      "seal": "d5266ebe6d0f5479dbc1efdea74425fd06ee1220",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/cd3a19d8a2492560c40ea10d453620dbd2ec5c55.cem.json"
    },
    {
      "path": ".corvint/changes/ceaaddaa9a323760cd605235789218282ddc8fe7.cem.json",
      "mode": "100644",
      "blob": "45db112dbf401096c791bc5323d5ca6edf969256",
      "sha256": "0a13c2e30f023f3a125decaf6854aa25d8025976e2c62cca2dda6209d843921d",
      "bind": "ceaaddaa9a323760cd605235789218282ddc8fe7",
      "seal": "1a5d734a5076193810c103b7d4bf16e82f363213",
      "sourceParent": "c90b81f28296627ef4d5d70b32464743cf4c62dc",
      "bindSharedMapByteEqual": true,
      "pureR100": true,
      "absentAtFrozenBase": true,
      "rawCopy": "/private/tmp/corvint-389-same-base-assessment-20261004/implementation-prep/preserved-raw/ceaaddaa9a323760cd605235789218282ddc8fe7.cem.json"
    }
  ],
  "qualification": "Ancestry-preserving tip-copy disposition only. New full-range CEM and checks required. Original pre-BASE unbound seed NOTE remains nonblocking but unqualified. Native docs-only and external outcome qualification remain open."
}
```
