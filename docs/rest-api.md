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

> Returns: `User Object`
<details>
<summary>View Payload Example</summary>

```json
{
    "id": "60c5600515668c9de42e6d69",
    "twitch_id": "",
    "login": "7tv_app",
    "display_name": "7tv_app",
    "role": {
        "id": "6102002eab1aa12bf648cfcd",
        "name": "Admin",
        "position": 76,
        "color": 14105645,
        "allowed": 64,
        "denied": 0
    }
}
```
</details>

### Get Emote
Get a single emote

> GET `/emotes/:emote`

> Returns: `Emote Object`
<details>
<summary>View Payload Example</summary>

```json
{
    "id": "60ae4a875d3fdae583c64313",
    "name": "FeelsDankMan",
    "owner": {
        "id": "60ae40fcaee2aa5538455d5a",
        "twitch_id": "",
        "login": "tomsomnium1",
        "display_name": "tomsomnium1",
        "role": {
            "id": "000000000000000000000000",
            "name": "",
            "position": 0,
            "color": 0,
            "allowed": 523,
            "denied": 0,
            "default": true
        }
    },
    "visibility": 0,
    "visibility_simple": [],
    "mime": "image/webp",
    "status": 3,
    "tags": [],
    "width": [
        27,
        41,
        65,
        110
    ],
    "height": [
        32,
        48,
        76,
        128
    ],
    "urls": [
        [
            "1",
            "https://cdn.7tv.app/emote/60ae4a875d3fdae583c64313/1x"
        ],
        [
            "2",
            "https://cdn.7tv.app/emote/60ae4a875d3fdae583c64313/2x"
        ],
        [
            "3",
            "https://cdn.7tv.app/emote/60ae4a875d3fdae583c64313/3x"
        ],
        [
            "4",
            "https://cdn.7tv.app/emote/60ae4a875d3fdae583c64313/4x"
        ]
    ]
}
```
</details>

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
