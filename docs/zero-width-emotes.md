# ZERO WIDTH EMOTES IMPLEMENTAION 

Zero-width emotes are supported on 7TV as part of the `visibility` and `visibility_simple` fields of the [Emote](https://github.com/SevenTV/ServerGo/blob/master/docs/rest-api.md#get-emote) type.

### Detecting

When requesting emote data from the API, there are two ways to tell whether an emote is zero width.

1. With the `visibility` value, bitfield flag `1 << 7`

	JS Example (Bitwise Operation)
	```js
		const zw = 1 << 7;
		const sum = emote.visibility;

		if ((sum & zw) == zw) {
			return true;
		}
	```

1. With the `visibility_simple` list of strings, if it contains `ZERO_WIDTH`

	JS Example
	```js
		const flags = emote.visibility_simple;

		if (flags.includes('ZERO_WIDTH')) {
			return true;
		}
	```

### Client-side Styling

The client should consider a zero width emote as having no bounding box, or a "zero" width value. 
CSS Example:
```css
.zero-width {
	width: 0px;
	position: absolute;
	z-index: 1;
}
```

The result shoud look similar to this:

![](https://cdn.discordapp.com/attachments/817075418640678964/870906527702188062/2021-07-31_07-51-29.gif)

### Testing

Currently, two emotes are defined as zero-width on the staging environment for testing purposes: ppBounce and wideppL, as global emotes.

Stage Base URL is `https://api-stage.7tv.app/v2`
