# WEBSOCKET API - DOCUMENTATION

This file documents the Websocket API.

## Reference

**UPGRADE URL**: `wss://api.7tv.app/v2/ws`

### Operation Codes
| Name | Code | Direction |
|------|------|-----------|
| Dispatch | 0 | Receive |
| Hello | 1 | Receive |
| Heartbeat | 2 | Send |
| Heartbeat Ack | 3 | Receive |
| Identify | 4 | Send |
| Server Closure | 5 | Receive |
| Subscribe | 6 | Send

#### Event Channels
| Name | Type ID | Unique |
| -----|---------|--------|
| CHANNEL_EMOTES | 1 | No |

## Connecting & Maintaining

Once connected, the server will immediately send an event with opcode `1 (HELLO)`. The event's payload contains `heartbeat_interval`, which is how often your client should send heartbeats.


Sending a heartbeat is done via opcode `2 (HEARTBEAT)`, which, if successful, the server will respond with `3 (HEARTBEAT_ACK)` and continue to maintain the connection. The client is expected to continue sending heartbeats at interval until the client terminates its connection.

## Subscribing
After connecting, the client can now subscribe to event channels. This is done by sending opcode `6 (SUBSCRIBE)` with a payload containing `type (int)` and `params (map)`. 

Example:
```json
// Subscribe to channel type 1 (CHANNEL_EMOTES)
{"op":6,"d":{"type":1,"params":{"channel":"vadikus007"}}}
```
