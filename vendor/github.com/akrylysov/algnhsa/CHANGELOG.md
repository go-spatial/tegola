# Changelog

## [0.12.1] - 2019-09-26
### Added
- Fixed compatibility with Go versions older than 1.13.

## [0.12.0] - 2019-09-26
### Added
- ALB support (thanks @adimarco and @ARolek for feedback).

## [0.11.0] - 2019-03-18
### Added
- Go Modules support.

## [0.10] - 2019-02-03
### Changed
- Set RequestURI on request (@RossHammer).
- Unescape Path (@RossHammer).
- Multi-value header support implemented using APIGatewayProxyResponse.MultiValueHeaders.

## [0.9] - 2018-12-10
### Added
- Support multi-value query string parameters and headers in requests.

## [0.8] - 2018-07-29
### Added
- Workaround for API Gateway not supporting headers with multiple values (@mspiegel).

## [0.7] - 2018-06-08
### Added
- UseProxyPath option - strips the base path mapping when using a custom domain with API Gateway.

## [0.6] - 2018-05-30
### Changed
- Set Host header for requests (@rvdwijngaard).

## [0.5] - 2018-02-05
### Added
- Context support.
