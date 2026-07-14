## 1.0.85 (July 14, 2026)


## 1.0.84 (July 14, 2026)
  - fix: added platform to config rpc call
  - fix: clear the mod staging directory at the start of a sync
  - fix: make syncmods report partial progress, refuse a nil mod list, and stop losing mods on a failed install
  - feat!: replace the mod poll loop with a syncmods task
  - feat: install game feature mods into Mods/GameFeatures
  - feat: Updated install script
  - fix: skip the SF update while the server is running
  - feat: single serial task executor with lease renewal and release
  - fix!: run shutdown operations in declared order
  - fix!: agent never self-installs; split install from reinstall
  - feat: single mutex over SF install dir and process
  - fix: send a session id and reset the reconnect backoff on a live stream
  - fix: stop leaking a goroutine per task stream reconnect
  - feat!: subscribe to task stream instead of polling
  - chore: Update deps
  - feat: Added insecure flag

## 1.0.83 (July 09, 2026)


## 1.0.82 (July 08, 2026)


## 1.0.81 (July 08, 2026)


## 1.0.80 (July 08, 2026)
  - Merge pull request #1 from SatisfactoryServerManager/feature/rest-to-grpc-migration
  - fix: Fixed agent
  - fix: make public IP lookup resilient with fallbacks
  - feat: add debug logging to log stream sender
  - fix: run DepotDownloader directly instead of via PowerShell
  - fix: surface SF server install errors instead of always succeeding
  - feat: add --grpcinsecure flag for plaintext gRPC in containers
  - fix: use insecure gRPC credentials in development mode
  - chore: remove dead agent REST helpers, keep ping connectivity check
  - refactor: agent uses gRPC file client instead of REST
  - feat: agent gRPC file-transfer client with resume-from-offset
  - chore: bump ssmcloud-resources and migrate to proto/generated layout
  - feat: added log message when branch changed

## 1.0.79 (December 09, 2025)
  - fix: Fixed sending version and ip to backend

## 1.0.78 (December 02, 2025)
  - fix: Fixed file permissions on sf server

## 1.0.77 (December 02, 2025)
  - fix: Fixed docker entry script

## 1.0.76 (December 02, 2025)
  - feat: Updated agent install scripts
  - feat: send final state on cleanup
  - feat: Mod Handler
  - removed makefile
  - feat: Mod config work
  - feat: grpc structure changes
  - feat: Proto functions
  - feat: GRPC connection
  - feat: log line changes
  - feat: new log handling

## 1.0.75 (November 03, 2025)
  - fix: Fixed modReference

## 1.0.74 (August 18, 2025)
  - feat: Slim down mod state put request

## 1.0.73 (August 18, 2025)
  - ci: Fixed Go version

## 1.0.72 (August 18, 2025)
  - fix: Fixed depot downloader errors
  - feat: more debug on docker install script

## 1.0.71 (January 02, 2025)
  - fix: Fixed entry script

## 1.0.70 (January 02, 2025)


## 1.0.69 (December 16, 2024)
  - fix: Fixed mod installation

## 1.0.68 (September 26, 2024)
  - feat: New save sync system

## 1.0.67 (September 25, 2024)
  - fix: Fixed update on start setting
  - fix: Fixes to auto restart
  - feat: Better steamcmd logging

## 1.0.66 (September 24, 2024)
  - feat:Switch to targz backup files

## 1.0.65 (September 13, 2024)
  - fix: Fixed docker cleanup script

## 1.0.64 (September 12, 2024)
  - feat: New install process
  - fix:fixes to install script

## 1.0.63 (September 11, 2024)
  - ci: Fixed download artifact

## 1.0.62 (September 11, 2024)
  - ci: Fixed uploading artifact

## 1.0.61 (September 11, 2024)


## 1.0.60 (September 11, 2024)


## 1.0.59 (September 11, 2024)
  - ci: Fixed CI

## 1.0.58 (September 11, 2024)
  - ci: Fixed CI

## 1.0.57 (September 11, 2024)
  - fix: Fixes to server filename change

## 1.0.56 (May 08, 2024)
  - fix: Fixed downloading save task

## 1.0.55 (May 07, 2024)
  - fix: added task processing debug
  - fix: Install scripts

## 1.0.54 (March 18, 2024)
  - chore: Bump Version
  - feat: update install script

## 1.0.53 (March 18, 2024)
  - fix: Fix mod config data
  - feat: Updates for new api change

## 1.0.52 (January 18, 2024)
  - feat: Set scalability config file

## 1.0.51 (January 18, 2024)
  - feat: Seasonal Events and auto save interval options

## 1.0.50 (January 16, 2024)
  - feat: Better game config management
  - fix install script
  - feat: Updated install script

## 1.0.49 (January 15, 2024)
  - chore: Bump Version

## 1.0.48 (January 15, 2024)
  - feat: More Server settings and mod configs
  - fix: Better error handling of ini files
  - fix: Dont allow changing sml when sf is running

## 1.0.47 (January 11, 2024)
  - feat: Added auto restart feature

## 1.0.46 (January 11, 2024)
  - fix: Fixes to startup args and gha pipeline

