## 0.7.3 (2016-04-10)

- Make replace case-insensitive. Fixes #167
- Fix purge logic for NS records (thanks @floppym) 
- Add sha256 checksums to releases 

## 0.7.2 (2016-03-31)

- Add --subdivision-code. Thanks @bensie.

## 0.7.1 (2016-03-19)

- Handle multiple values for SPF/TXT records correctly. Fixes #160.
- Fix delete/replace of wildcard records. Fixes #150.

## 0.7.0 (2016-03-05)

- Add support to rrcreate for creating multiple records
- Correct MX example in docs. Closes #154
- Disable debug info from release builds => significantly smaller executables

## 0.6.9 (2016-02-18)

- Warn and skip traffic policy records

## 0.6.8 (2016-01-17)

- Handle importing aliases and alias target simultaneously correctly. Fixes #133
- Add VPC private zone create support. Fixes #122
- Leave alias target expanded for 'export --full'. Fixes #132

## 0.6.7 (2015-12-27)

- Fix quoting SPF record support. Fixes #138.  (tag: 0.6.7)

## 0.6.6 (2015-12-12)

- Fix comparison of wildcard records on 'import --replace'. Fixes #127
- Add more ALIAS examples. Issue #129
- Add docs on CNAME trailing dot. Fixes #124

## 0.6.5 (2015-11-09)

- Fix CNAMEs to origin. Fixes #123

## 0.6.4 (2015-11-08)

- Add --profile option to select credentials. Fixes #117.
- Sort exported records by name, SOA, then other types. Fixes #121.
- Add GO15VENDOREXPERIMENT=1 in 'Building from source'. Fixes #120.

## 0.6.3 (2015-10-24)

- Add codecov.
- Support for wildcard records
- Parameter validation
- Add --replace for rrcreate.
- Allow zero weighted records.

## 0.6.2 (2015-10-14)

- README additions.
- Allow domain name with final period on command line.
- Paginate when finding a zone.
- Fix pagination bug with multiple records under same name. Fixes #112

## 0.6.1 (2015-10-13)

- Remove win64 build from upx.
- Ensure throttled requests in tests are retried.
- Fix goupx

## 0.6.0 (2015-10-13)

- Go!
