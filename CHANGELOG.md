# Changelog

## [v1.0.0](https://github.com/apricote/grafana-hcloud-datasource/releases/tag/v1.0.0)

### Breaking Changes

All dependencies of the plugin have been updated, and it now requires Grafana 12.1&#43;.

### Features

- **BREAKING**: Grafana 12 (#13)

### Bug Fixes

- aborted request when API returned invalid_input error (#12)

## 0.3.0 (2024-04-28)

### New Features

- Colors in time series visualisation are now consistent across refreshes

## 0.2.0 (2024-01-12)

### New Features

- Sign Plugin for distribution
- improve error message when API Token is invalid or missing

### Bug Fixes

- resource calls using wrong paths and not handling unknown route

## 0.1.1 (2024-01-10)

### Bug Fixes

- remove double slash in resource calls from frontend
- remove type assertation for *prometheus.Registry by @apricote in #1


## 0.1.0 (2024-01-06)

- Initial release.
