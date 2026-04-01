# Changelog

## 0.2.1

- Standardize README to 3-badge format with emoji Support section
- Update CI checkout action to v5 for Node.js 24 compatibility
- Add GitHub issue templates, dependabot config, and PR template

## 0.2.0

- Add `SendJSON` and `BroadcastJSON` methods for JSON-serialized SSE events
- Add topic-based routing with `Subscribe` and `PublishTopic` methods
- Add `OnConnect` and `OnDisconnect` lifecycle callbacks
- Topic subscriptions are automatically cleaned up on client disconnect

## 0.1.3

- Consolidate README badges onto single line, fix CHANGELOG format

## 0.1.2

- Add Development section to README

## 0.1.0

- Initial release
- SSE broker with client management and broadcast
- Event builder with id, event, data, retry fields
- SSE client for consuming streams
- Automatic keep-alive pings
