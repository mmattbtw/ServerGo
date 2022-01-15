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
    },
    "profile_picture_id":"c8585ca40aeb48b9b474b8cab99b93e1"
}
```
</details>

<details>
<summary>Additional Notes</summary>

### Animated Profile Pictures
Animated Profile Pictures set on the 7tv website (only available to 7tv Subscribers) is returned in the REST API as `profile_picture_id`. You can use this to find the Animated Profile Picture's URL: `https://cdn.7tv.app/pp/{id}/{profile_picture_id}`. This isn't always returned, as not everyone has an Animated Profile Picture. 

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

### Get Cosmetics

Get all active cosmetics

> GET `/cosmetics`

> Query: `user_identifier: "object_id", "twitch_id", or "login"`

> Returns: `Lists of Badges and Paints Objects`

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
	],
	"paints": [
		{
			"id": "61bede3db6b41ea54419bbb0",
			"name": "Candy Cane",
			"users": [
				"24377667"
			],
			"function": "linear-gradient",
			"color": -10197761,
			"stops": [
				{
					"at": 0.1,
					"color": -757935361
				},
				{
					"at": 0.2,
					"color": -757935361
				},
				{
					"at": 0.2,
					"color": -10197761
				},
				{
					"at": 0.3,
					"color": -10197761
				}
			],
			"repeat": true,
			"angle": 45,
			"drop_shadows": [
				{
					"x_offset": 0,
					"y_offset": 0,
					"radius": 0,
					"color": 0
				}
			]
		},
		{
			"id": "61c01b08b6b41ea54419bbbd",
			"name": "Staff Shine",
			"users": [
				"24377667"
			],
			"function": "url",
			"color": -97373441,
			"stops": [],
			"repeat": false,
			"angle": 0,
			"image_url": "https://cdn.7tv.app/misc/img_paints/img_paint_clip_test.webp",
			"drop_shadows": [
				{
					"x_offset": 0,
					"y_offset": 0,
					"radius": 4,
					"color": 100
				}
			]
		}
	]
}
```
</details>

<details>
<summary>Additional Notes</summary>

### Color
 The color property of a paint is a nullable 32-bit integer value representing a RGBA value. If null, then use the user's Twitch color. This can be decoded using the following example (for Javascript/Typescript):
 
 ```ts
	function decimalColorToRGBA(num: number): string {
		const r = (num >>> 24) & 0xFF;
		const g = (num >>> 16) & 0xFF;
		const b = (num >>> 8) & 0xFF;
		const a = num & 0xFF;

		return `rgba(${r}, ${g}, ${b}, ${(a / 255).toFixed(3)})`;
	}
 ```

### Function
The function property of a paint can be either `linear-gradient`, `radial-gradient`, or `url`. If `url`, then use the image_url property.

</details>
