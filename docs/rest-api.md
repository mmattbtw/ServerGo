# PUBLIC REST API - DOCUMENTATION

This file documments the public REST API for interfacing with 7TV.

## Reference

**BASE URL:** `https://api.7tv.app/v2`

### Versions

| Version | Status  | Default |
|---------|---------|---------|
| v2      | Online  | No      |
| v1      | Defunct | Yes     |

## Routes

### Get User
> GET `/users/:user`

### Get Emote
> GET `/emotes/:emote`

> Returns: `Emote Object`

Get a single emote

### Get Channel Emotes
> GET `/users/:user/emotes`

> Returns: `List of Emote Objects`


Get a user's active channel emotes

### Get Global Emotes
> GET `/emotes/global`

> Returns: `List of Emote Objects`

Get all current global emotes.

### Get Badges
> GET `/badges`

> Returns: `List of Badge Objects`

Get all active badges

### Add Channel Emote
> PUT `/users/:user/emotes/:emote`

> Authentication Required: **`YES`**

> Returns: `Emote Object and List of ObjectiD`

Enable an emote for a user's channel

### Remove Channel Emote
> DELETE `/users/:user/emotes/:emote`

> Authentication Required: **`YES`**

> Returns: `Void`


Remove an emote for a user's channel