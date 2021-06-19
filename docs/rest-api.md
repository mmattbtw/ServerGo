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
Get a single user

> GET `/users/:user`

### Get Emote
Get a single emote

> GET `/emotes/:emote`

> Returns: `Emote Object`

### Get Channel Emotes
Get a user's active channel emotes

> GET `/users/:user/emotes`

> Returns: `List of Emote Objects`

### Get Global Emotes
Get all current global emotes.

> GET `/emotes/global`

> Returns: `List of Emote Objects`

### Get Badges
Get all active badges

> GET `/badges`

> Query: `user_identifier: "object_id", "twitch_id", or "login"`

> Returns: `List of Badge Objects`
<details>
<summary>View Payload Example</summary>

```json
{
	"badges": [
		{
			"id": "60cd6255a4531e54f76d4bd4",
			"name": "Admin",
			"tooltip": "7TV Admin",
			"urls": [
				[
					"1",
					"https://cdn.7tv.app/badge/60cd6255a4531e54f76d4bd4/1x",
					""
				],
				[
					"2",
					"https://cdn.7tv.app/badge/60cd6255a4531e54f76d4bd4/2x",
					""
				],
				[
					"3",
					"https://cdn.7tv.app/badge/60cd6255a4531e54f76d4bd4/3x",
					""
				]
			],
			"users": [
				"24377667"
			]
		}
	]
}
```
</details>

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
